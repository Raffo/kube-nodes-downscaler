package main

import (
	"fmt"
	"log"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

type autoscalingInterface interface {
	SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error)
	DescribeAutoScalingInstances(input *autoscaling.DescribeAutoScalingInstancesInput) (*autoscaling.DescribeAutoScalingInstancesOutput, error)
	DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

const (
	sleepSeconds = 60
)

// ASG is the basic data type to deal with AWS ASGs.
type ASG struct {
	Name   string
	Client autoscalingInterface
}

// SetCapacity sets the capacity of the ASG to "capacity"
func (a *ASG) SetCapacity(capacity int64) error {
	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(a.Name),
		DesiredCapacity:      aws.Int64(capacity),
		HonorCooldown:        aws.Bool(true),
	}

	_, err := a.Client.SetDesiredCapacity(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return aerr
		}
		return err
	}
	return nil
}

// GetCurrentCapacity fetches the current capacity of the ASG given its name.
func (a *ASG) GetCurrentCapacity() (int, error) {
	out, err := a.Client.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{aws.String(a.Name)}})
	if err != nil {
		return -1, fmt.Errorf("cannot get current size of autoscaling group: %v", err)
	}
	return int(*out.AutoScalingGroups[0].DesiredCapacity), nil
}

func autodetectASGName(client autoscalingInterface, instanceName *string) (string, error) {
	out, err := client.DescribeAutoScalingInstances(&autoscaling.DescribeAutoScalingInstancesInput{InstanceIds: []*string{
		instanceName,
	}})
	if err != nil {
		return "", err
	}
	instances := out.AutoScalingInstances
	if len(instances) != 1 {
		return "", fmt.Errorf("wrong size of autoscaling instances, expected 1, have %d", len(instances))
	}
	return *instances[0].AutoScalingGroupName, nil
}

func determineNewCapacity(startTime, endTime, cap, day, currentHour int, consultantMode bool) int {
	if cap > 0 {
		if currentHour > endTime {
			// scale down to 0
			return 0
		}
	} else {
		if !consultantMode {
			if day == 6 || day == 7 {
				return cap
			}
		}
		if currentHour > startTime {
			// scale up
			return 2
		}
	}
	return cap
}

func main() {
	startTime := kingpin.Flag("start", "Start of the working day. 24h format.").Default("9").Int()
	endTime := kingpin.Flag("end", "End of the working day. 24h format.").Default("18").Int()
	consultantMode := kingpin.Flag("consultant-mode", "When true, will make sure that the nodes are available during the weekend.").Default("false").Bool()
	asgName := kingpin.Flag("asg-name", "Name of the autoscaling group. Useful to make the downscaler handle different ASGs from the one it's running on.").String()
	autoDetectASG := kingpin.Flag("autodetect", "Autodetect ASG group name, which is the ASG where this application is running.").Bool()
	kingpin.Parse()

	session := session.New()

	svc := ec2metadata.New(session)
	id, err := svc.GetInstanceIdentityDocument()
	region := id.Region
	if err != nil {
		log.Fatalf("Cannot get identity document: %v\n", err)
	}
	client := autoscaling.New(session, aws.NewConfig().WithRegion(region))
	if *autoDetectASG == true {
		asg, err := autodetectASGName(client, &id.InstanceID)
		if err != nil {
			log.Fatalf("Cannot get ASG name: %v\n", err)
		}
		*asgName = asg
	}

	if *asgName == "" {
		log.Fatalf("No ASG name provided, exiting.\n")
	}

	asg := ASG{
		Name:   *asgName,
		Client: client,
	}

	if *startTime < 1 || *startTime > 24 {
		log.Fatalf("Start of working day should be greater or equal than 1 and less than 24, have: %d\n", *startTime)
	}
	if *endTime < 1 || *endTime > 24 {
		log.Fatalf("End of working day should be greater or equal than 1 and less than 24, have: %d\n", *endTime)
	}

	if *endTime < *startTime {
		log.Fatalf("End of working day %d should be greater than start %d\n", *endTime, *startTime)
	}

	log.Println("starting the loop")
	for {
		t := time.Now()
		day := t.Weekday()
		cap, err := asg.GetCurrentCapacity()
		if err != nil {
			log.Fatalf("error getting current ASG capacity: %v", err)
		}
		newCap := determineNewCapacity(*startTime, *endTime, cap, int(day), t.Hour(), *consultantMode)
		if newCap != cap {
			log.Printf("setting capacity to %d, previous: %d\n", newCap, cap)
			err := asg.SetCapacity(int64(newCap))
			if err != nil {
				//deciding to fail fast here. As this code is designed to work only with Kubernetes we get restarts for free.
				log.Fatalf("error setting ASG capacity: %v", err)
			}
		}
		log.Printf("Nothing left to do, going to sleep for %d seconds\n", sleepSeconds)
		time.Sleep(sleepSeconds * time.Second)
	}
}
