package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	instanceIdFlag = kingpin.Flag("instance-id", "Instance id to mark").String()
	stackNameFlag  = kingpin.Flag("stack-name", "Stack resource is a part of").String()
	resourceIdFlag = kingpin.Flag("resource-id", "Id for the resource").String()
	timeout        = kingpin.Flag("timeout", "Timeout, after which if no success it will fail").Default("5m").Duration()
	commandSleep   = kingpin.Flag("command-sleep", "Time to sleep inbetween command executions").Default("10s").Duration()

	command = kingpin.Arg("command", "Command to execute").Required().String()
	args    = kingpin.Arg("arguments", "arguments to pass to command").Strings()
)

func getResourceTagValue(client *ec2.EC2, id, tag string) (string, error) {
	resp, err := client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(id)},
			},
			{
				Name:   aws.String("key"),
				Values: []*string{aws.String(tag)},
			},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Tags) > 0 {
		return *resp.Tags[0].Value, nil
	}
	return "", fmt.Errorf("Couldn't find the tag '%s' for resource '%s'", tag, id)
}

func commandUntilSuccess(out chan bool, sleep time.Duration, command string, args []string) {
	for {
		cmd := exec.Command(command, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err == nil {
			out <- true
			return
		} else {
			fmt.Printf("Command failed: %s\n", err)
		}

		time.Sleep(sleep)
	}
}

func main() {
	kingpin.Parse()

	sess := session.Must(session.NewSession())

	var instanceId, stackName, resourceId string
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

	ec2Client := ec2.New(sess)

	if stackNameFlag != nil && *stackNameFlag != "" {
		stackName = *stackNameFlag
	} else {
		if stackName, err = getResourceTagValue(ec2Client, instanceId, "aws:cloudformation:stack-name"); err != nil {
			panic(err)
		}
	}

	if resourceIdFlag != nil && *resourceIdFlag != "" {
		resourceId = *resourceIdFlag
	} else {
		if resourceId, err = getResourceTagValue(ec2Client, instanceId, "aws:cloudformation:logical-id"); err != nil {
			panic(err)
		}
	}

	commandSuccessCh := make(chan bool)
	timeoutCh := time.After(*timeout)

	go commandUntilSuccess(commandSuccessCh, *commandSleep, *command, *args)

	health := "SUCCESS"

	select {
	case <-commandSuccessCh:
	case <-timeoutCh:
		health = "FAILURE"
	}

	cloudformationClient := cloudformation.New(sess)

	_, err = cloudformationClient.SignalResource(&cloudformation.SignalResourceInput{
		LogicalResourceId: &resourceId,
		StackName:         &stackName,
		Status:            &health,
		UniqueId:          &instanceId,
	})

	if err != nil {
		panic(err)
	}
}
