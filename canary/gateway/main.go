package main

import (
	"flag"
	"os"

	"golang.org/x/net/context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	myDefaultKbConfig := os.Getenv("PATH_KUBECONFIG")
	kubeconfig := flag.String("kubeconfig", myDefaultKbConfig, "kubeconfig file")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	gatewayGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "gateways",
	}

	gateway := &unstructured.Unstructured{}
	gateway.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "networking.istio.io/v1alpha3",
		"kind":       "Gateway",
		"metadata": map[string]interface{}{
			"name": "hello-app-gateway",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"istio": "ingressgateway",
			},
			"servers": []map[string]interface{}{
				{
					"port": map[string]interface{}{
						"number":   80,
						"name":     "http",
						"protocol": "HTTP",
					},
					"hosts": []string{
						"*",
					},
				},
			},
		},
	})

	_, err = dynamicClient.Resource(gatewayGVR).Namespace("default").Create(ctx, gateway, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
}
