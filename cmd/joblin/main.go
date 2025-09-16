package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"go.etcd.io/bbolt"
	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/stretchr/testify/assert"
)

// Temporary main function to enable dependency resolution
func main() {
	fmt.Println("Joblin CLI - under development")

	// Reference imports to ensure they're included in go.mod
	_ = cobra.Command{}
	_ = viper.New()
	_ = logrus.New()
	_ = kubernetes.Clientset{}
	_ = rest.Config{}
	_ = runtime.Object(nil)
	_ = bbolt.DB{}
	_ = goteamsnotify.NewClient()
	_ = assert.Equal
}