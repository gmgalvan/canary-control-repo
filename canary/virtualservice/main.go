// https://istio.io/latest/blog/2019/announcing-istio-client-go/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	inetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	iv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	istioScheme "istio.io/client-go/pkg/clientset/versioned/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	ctx := context.Background()

	// get the config rest
	myDefaultKbConfig := os.Getenv("PATH_KUBECONFIG")
	kubeconfig := flag.String("kubeconfig", myDefaultKbConfig, "kubeconfig file")
	flag.Parse()

	// get configigs.k8s.io/controller-runtime/pkg/cl
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// Create runtime client
	crScheme := runtime.NewScheme()

	/*
		Schema:
		Golang type -> GVK(Group Version KInd)
	*/
	// istio
	if err = istioScheme.AddToScheme(crScheme); err != nil {
		fmt.Println(err)
	}

	// kubernetes native objects (service)
	if err = clientgoscheme.AddToScheme(crScheme); err != nil {
		fmt.Println(err)
	}

	// client set
	cl, err := runtimeclient.New(config, runtimeclient.Options{
		Scheme: crScheme,
	})
	if err != nil {
		panic(err.Error())
	}

	//service creation
	helloAppService := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-app",
			Namespace: "default", // if you want to specify a namespace
			Labels: map[string]string{
				"app":     "hello-app",
				"service": "hello-app",
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "hello-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	// Create or update the service
	existingService := &v1.Service{}
	err = cl.Get(ctx, types.NamespacedName{Namespace: "default", Name: "hello-app"}, existingService)
	if err != nil {
		// If service doesn't exist, create it
		err = cl.Create(ctx, helloAppService)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		// If service exists, update it
		helloAppService.ResourceVersion = existingService.ResourceVersion // Important for update to work
		err = cl.Update(ctx, helloAppService)
		if err != nil {
			fmt.Println(err)
		}
	}

	// create istio object
	vsHelloApp := &iv1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualService",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-app",
			Namespace: "default", // if you want to specify a namespace
		},
		Spec: inetworkingv1alpha3.VirtualService{
			Hosts:    []string{"*"},
			Gateways: []string{"hello-app-gateway"},
			Http: []*inetworkingv1alpha3.HTTPRoute{
				{
					Route: []*inetworkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &inetworkingv1alpha3.Destination{
								Host:   "hello-app",
								Subset: "hello-app-v1",
								Port: &inetworkingv1alpha3.PortSelector{
									Number: 80,
								},
							},
							Weight: 50,
						},
						{
							Destination: &inetworkingv1alpha3.Destination{
								Host:   "hello-app",
								Subset: "hello-app-v2",
								Port: &inetworkingv1alpha3.PortSelector{
									Number: 80,
								},
							},
							Weight: 50,
						},
					},
				},
			},
		},
	}

	// creating the istio virtual service
	err = cl.Create(ctx, vsHelloApp)
	if err != nil {
		fmt.Println(err)
	}

}
