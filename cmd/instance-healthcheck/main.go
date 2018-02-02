package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	instanceIdFlag = kingpin.Flag("instance-id", "Instance id to mark").String()

	command = kingpin.Arg("command", "Command to execute").Required().String()
	args    = kingpin.Arg("arguments", "arguments to pass to command").Strings()
)

func main() {
	kingpin.Parse()

	sess := session.Must(session.NewSession())

	var instanceId string
	var err error

	if instanceIdFlag != nil && *instanceIdFlag != "" {
		instanceId = *instanceIdFlag
	} else {
		metadata := ec2metadata.New(sess)

		if !metadata.Available() {
			fmt.Fprintf(os.Stderr, "Metadata isn't avaiable")
			os.Exit(1)
		}

		instanceId, err = metadata.GetMetadata("/latest/meta-data/instance-id")
		if err != nil {
			panic(err)
		}
	}

	cmd := exec.Command(*command, *args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	health := "Healthy"
	if err != nil {
		health = "Unhealthy"
	}

	autoscalingClient := autoscaling.New(sess)

	_, err = autoscalingClient.SetInstanceHealth(&autoscaling.SetInstanceHealthInput{
		HealthStatus: &health,
		InstanceId:   &instanceId,
	})

	if err != nil {
		panic(err)
	}
}
