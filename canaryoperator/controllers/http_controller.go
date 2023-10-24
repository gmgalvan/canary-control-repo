/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	canaryv1alpha1 "github.com/canary/api/v1alpha1"

	istionetworkv1alpha3 "istio.io/api/networking/v1alpha3"
	istiov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// HttpReconciler reconciles a Http object
type HttpReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=canary.demo.com,resources=https,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=canary.demo.com,resources=https/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=canary.demo.com,resources=https/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Http object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *HttpReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch canaryDeploy
	canaryDeploy := &canaryv1alpha1.Http{}
	err := r.Get(ctx, req.NamespacedName, canaryDeploy)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("TfCanary resource not found. Ignoring since object must be delete")
			return ctrl.Result{}, nil
		}
	}

	// Check if Deployments already exists if not create a new one
	for _, serviceDep := range canaryDeploy.Spec.Deployment {
		// Check if the deployment already exists, if not create a new one
		found := &appsv1.Deployment{}
		deployedModelName := canaryDeploy.Name + "-" + canaryDeploy.Name
		err = r.Get(ctx, types.NamespacedName{Name: deployedModelName, Namespace: canaryDeploy.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			// Define a new deployment
			dep := r.modelDeployment(ctx, *canaryDeploy, serviceDep)
			log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			err = r.Create(ctx, dep)
			if err != nil {
				log.Error(err, "Failed to create new Model Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
				return ctrl.Result{}, err
			}
			//return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "Failed to get Deployment")
			return ctrl.Result{}, err
		}
	}

	// Check if Services already exists if not create a new one
	svcFound := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: canaryDeploy.Name, Namespace: canaryDeploy.Namespace}, svcFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		svc := r.modelService(ctx, *canaryDeploy)
		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Model Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		//return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Check if Gateway already exists
	gatewayFound := &istiov1alpha3.Gateway{}
	gatewayName := canaryDeploy.Name + "-gateway"
	err = r.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: canaryDeploy.Namespace}, gatewayFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new gateway
		gtw := r.ensureGateway(*canaryDeploy)
		log.Info("Creating a new gateway", "gateway.Namespace", gtw.Namespace, "gateway.Name", gtw.Name)
		err = r.Create(ctx, gtw)
		if err != nil {
			log.Error(err, "Failed to create new gateway", "gateway.Namespace", gtw.Namespace, "gateway.Name", gtw.Name)
			return ctrl.Result{}, err
		}
		// gateway created successfully - return and requeue
		//return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Gateway")
		return ctrl.Result{}, err
	}

	// Check for istio destination rule
	destinationRulesFound := &istiov1alpha3.DestinationRule{}
	err = r.Get(ctx, types.NamespacedName{Name: canaryDeploy.Name, Namespace: canaryDeploy.Namespace}, destinationRulesFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new destination rule
		dr := r.createDestinationRules(*canaryDeploy)
		log.Info("Creating a new destination rule", "destinationRule.Namespace", dr.Namespace, "destinationRule.Name", dr.Name)
		err = r.Create(ctx, dr)
		if err != nil {
			log.Error(err, "Failed to create new destination rule", "destinationRule.Namespace", dr.Namespace, "destinationRule.Name", dr.Name)
			return ctrl.Result{}, err
		}
		// destination rule created successfully - return and requeue
		//return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get destination rule")
		return ctrl.Result{}, err
	}

	// check for virtual service config
	virtualServiceFound := &istiov1alpha3.VirtualService{}
	err = r.Get(ctx, types.NamespacedName{Name: canaryDeploy.Name, Namespace: canaryDeploy.Namespace}, virtualServiceFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new virtual service
		vs := r.createWeightedVirtualService(*canaryDeploy)
		log.Info("Creating a new virtual service", "virtualService.Namespace", vs.Namespace, "virtualService.Name", vs.Name)
		err = r.Create(ctx, vs)
		if err != nil {
			log.Error(err, "Failed to create new virtual service", "virtualService.Namespace", vs.Namespace, "virtualService.Name", vs.Name)
			return ctrl.Result{}, err
		}
		// virtual service created successfully - return and requeue
		//return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get virtual service")
		return ctrl.Result{}, err
	}
	// update
	deploymentsFound := &appsv1.DeploymentList{}
	// set a list of options from metav1
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": canaryDeploy.Name}}
	err = r.Client.List(ctx, deploymentsFound, &client.ListOptions{Namespace: canaryDeploy.Namespace, LabelSelector: labels.Set(labelSelector.MatchLabels).AsSelector()})
	if err != nil {
		log.Info("an error occours while trying to get deployments")
	}
	// support only weight change
	for idx := range deploymentsFound.Items {
		if canaryDeploy.Spec.Deployment[idx].Weight != virtualServiceFound.Spec.Http[0].Route[idx].Weight {
			// update all in order to ensure that sums 100
			for j, _ := range virtualServiceFound.Spec.Http[0].Route {
				virtualServiceFound.Spec.Http[0].Route[j].Weight = canaryDeploy.Spec.Deployment[j].Weight
			}
			err = r.Update(ctx, virtualServiceFound)
			if err != nil {
				log.Error(err, "Failed to update weight", "Deployment.Namespace", canaryDeploy.Namespace, "Deployment.Name", canaryDeploy.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *HttpReconciler) createWeightedVirtualService(canaryDepHttp canaryv1alpha1.Http) *istiov1alpha3.VirtualService {
	gatewayName := canaryDepHttp.Name + "-gateway"
	routeDestination := []*istionetworkv1alpha3.HTTPRouteDestination{}
	for _, depServ := range canaryDepHttp.Spec.Deployment {
		routeDestination = append(routeDestination, &istionetworkv1alpha3.HTTPRouteDestination{
			Destination: &istionetworkv1alpha3.Destination{
				Host: canaryDepHttp.Name,
				Port: &istionetworkv1alpha3.PortSelector{
					Number: 8501,
				},
				Subset: canaryDepHttp.Name,
			},
			Weight: depServ.Weight,
		})
	}
	vs := &istiov1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			Kind: "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      canaryDepHttp.Name,
			Namespace: canaryDepHttp.Namespace,
		},
		Spec: istionetworkv1alpha3.VirtualService{
			Hosts:    []string{"*"},
			Gateways: []string{gatewayName},
			Http: []*istionetworkv1alpha3.HTTPRoute{
				{
					Route: routeDestination,
				},
			},
		},
	}
	ctrl.SetControllerReference(&canaryDepHttp, vs, r.Scheme)
	return vs
}

func (r *HttpReconciler) createDestinationRules(canaryDepHttp canaryv1alpha1.Http) *istiov1alpha3.DestinationRule {
	subsets := []*istionetworkv1alpha3.Subset{}
	for _, depServ := range canaryDepHttp.Spec.Deployment {
		subsets = append(subsets, &istionetworkv1alpha3.Subset{
			Name:   canaryDepHttp.Name,
			Labels: map[string]string{"version": depServ.Version},
		})
	}
	dr := &istiov1alpha3.DestinationRule{
		TypeMeta: metav1.TypeMeta{
			Kind: "DestinationRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      canaryDepHttp.Name,
			Namespace: canaryDepHttp.Namespace,
		},
		Spec: istionetworkv1alpha3.DestinationRule{
			Host:    canaryDepHttp.Name,
			Subsets: subsets,
		},
	}
	ctrl.SetControllerReference(&canaryDepHttp, dr, r.Scheme)
	return dr
}

func (r *HttpReconciler) ensureGateway(canaryDepHttp canaryv1alpha1.Http) *istiov1alpha3.Gateway {
	gateway := &istiov1alpha3.Gateway{
		TypeMeta: metav1.TypeMeta{
			Kind: "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      canaryDepHttp.Name + "-gateway",
			Namespace: "default",
		},
		Spec: istionetworkv1alpha3.Gateway{
			Selector: map[string]string{"istio": "ingressgateway"},
			Servers: []*istionetworkv1alpha3.Server{
				{
					Port: &istionetworkv1alpha3.Port{
						Number:   80,
						Name:     "http",
						Protocol: "HTTP",
					},
					Hosts: []string{"*"},
				},
			},
		},
	}
	ctrl.SetControllerReference(&canaryDepHttp, gateway, r.Scheme)
	return gateway
}

func (r *HttpReconciler) modelService(ctx context.Context, canaryDepHttp canaryv1alpha1.Http) *corev1.Service {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      canaryDepHttp.Name,
			Namespace: canaryDepHttp.Namespace,
			Labels: map[string]string{
				"app":     canaryDepHttp.Name,
				"service": canaryDepHttp.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": canaryDepHttp.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	ctrl.SetControllerReference(&canaryDepHttp, svc, r.Scheme)
	return svc
}

func (r *HttpReconciler) modelDeployment(ctx context.Context, canaryDepHttp canaryv1alpha1.Http, serviceDep canaryv1alpha1.Deployment) *appsv1.Deployment {
	nameBasePath := fmt.Sprintf("%v-deployment-%v", canaryDepHttp.Name, serviceDep.Version)

	depService := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: canaryDepHttp.Namespace,
			Name:      nameBasePath,
			Labels: map[string]string{
				"app":     canaryDepHttp.Name,
				"version": serviceDep.Version,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     canaryDepHttp.Name,
					"version": serviceDep.Version,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     canaryDepHttp.Name,
						"version": serviceDep.Version,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  canaryDepHttp.Name,
							Image: serviceDep.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(&canaryDepHttp, depService, r.Scheme)
	return depService
}

// SetupWithManager sets up the controller with the Manager.
func (r *HttpReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&canaryv1alpha1.Http{}).
		Complete(r)
}

func int32Ptr(i int32) *int32 { return &i }
