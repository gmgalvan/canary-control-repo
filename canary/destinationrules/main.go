package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	inetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istiov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	// get the config rest
	myDefaultKbConfig := os.Getenv("PATH_KUBECONFIG")
	kubeconfig := flag.String("kubeconfig", myDefaultKbConfig, "kubeconfig file")
	flag.Parse()

	// get config
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// ensure istio versioned clients set
	ic, err := versionedclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create istio client: %s", err)
	}

	dr := &istiov1alpha3.DestinationRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DestinationRule",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-app",
			Namespace: "default",
		},
		Spec: inetworkingv1alpha3.DestinationRule{
			Host: "hello-app",
			Subsets: []*inetworkingv1alpha3.Subset{
				{
					Name:   "hello-app-v1",
					Labels: map[string]string{"version": "v1"},
				},
				{
					Name:   "hello-app-v2",
					Labels: map[string]string{"version": "v2"},
				},
			},
		},
	}

	// Create the destination rule
	_, err = ic.NetworkingV1alpha3().DestinationRules("default").Create(ctx, dr, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
	}
}
