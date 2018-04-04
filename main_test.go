package main

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

const (
	asgName = "asgName"
)

type MockAutoscalingClient struct {
	DesiredCapacity int
	EmulatedErr     string
}

func (m *MockAutoscalingClient) SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	if m.EmulatedErr != "" {
		return nil, awserr.New(m.EmulatedErr, "error", nil)
	}
	m.DesiredCapacity = int(*input.DesiredCapacity)
	return nil, nil
}

func (m *MockAutoscalingClient) getMockCapacity() int {
	return m.DesiredCapacity
}

func (m *MockAutoscalingClient) DescribeAutoScalingInstances(input *autoscaling.DescribeAutoScalingInstancesInput) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	return &autoscaling.DescribeAutoScalingInstancesOutput{AutoScalingInstances: []*autoscaling.InstanceDetails{&autoscaling.InstanceDetails{AutoScalingGroupName: aws.String(asgName)}}}, nil
}

func (m *MockAutoscalingClient) DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []*autoscaling.Group{&autoscaling.Group{DesiredCapacity: aws.Int64(int64(m.DesiredCapacity))}}}, nil
}

func NewMockASG() *ASG {
	asg := &ASG{
		Name: asgName,
	}
	return asg
}

func TestSetCapacity(t *testing.T) {
	client := &MockAutoscalingClient{}
	asg := NewMockASG()
	asg.Client = client
	newCap := 3
	err := asg.SetCapacity(int64(newCap))
	if err != nil {
		t.Fatalf("error while setting capacity: %v", err)
	}
	cap := client.getMockCapacity()
	if cap != newCap {
		t.Fatalf("expected %d, got %d", 3, cap)
	}

}

func TestSetCapacityWithActivityInProgress(t *testing.T) {
	client := &MockAutoscalingClient{EmulatedErr: autoscaling.ErrCodeScalingActivityInProgressFault}
	asg := NewMockASG()
	asg.Client = client
	newCap := 3
	err := asg.SetCapacity(int64(newCap))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() != autoscaling.ErrCodeScalingActivityInProgressFault {
			t.Fatalf("expecting %v, got %v", autoscaling.ErrCodeScalingActivityInProgressFault, aerr.Code())
		}
	}
}

func TestSetCapacityWithContentionFault(t *testing.T) {
	client := &MockAutoscalingClient{EmulatedErr: autoscaling.ErrCodeResourceContentionFault}
	asg := NewMockASG()
	asg.Client = client
	newCap := 3
	err := asg.SetCapacity(int64(newCap))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() != autoscaling.ErrCodeResourceContentionFault {
			t.Fatalf("expecting %v, got %v", autoscaling.ErrCodeResourceContentionFault, aerr.Code())
		}
	}
}

func TestGetCurrentCapacity(t *testing.T) {
	refCap := 2
	client := &MockAutoscalingClient{DesiredCapacity: refCap}
	asg := NewMockASG()
	asg.Client = client
	cap, err := asg.GetCurrentCapacity()
	if err != nil {
		t.Fatalf("error while getting capacity: %v", err)
	}
	if cap != refCap {
		t.Fatalf("expected %d, got %d", refCap, cap)
	}
}

func TestAutodetectASGName(t *testing.T) {
	instanceName := "i-123456789"
	client := &MockAutoscalingClient{}
	name, err := autodetectASGName(client, &instanceName)
	if err != nil {
		t.Fatalf("error while getting ASG name: %v", err)
	}
	if name != asgName {
		t.Fatalf("expected %s, got %s", asgName, name)
	}
}

func TestDetermineNewCapacity(tt *testing.T) {

	for _, test := range []struct {
		name           string
		startTime      int
		endTime        int
		cap            int
		day            int
		currentHour    int
		consultantMode bool
		expectedCap    int
	}{
		{
			name:           "During working hours in the mid of the week the cluster should keep its capacity.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            2,
			currentHour:    12,
			consultantMode: false,
			expectedCap:    2,
		},
		{
			name:           "After working hours in the mid of the week the cluster should scale down.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            2,
			currentHour:    19,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "During the weekend during the day, the cluster should stay scaled down.",
			startTime:      9,
			endTime:        18,
			cap:            0,
			day:            6,
			currentHour:    19,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "During the weekend with consultant mode the cluster should scale up.",
			startTime:      9,
			endTime:        18,
			cap:            0,
			day:            6,
			currentHour:    10,
			consultantMode: true,
			expectedCap:    2,
		},
		{
			name:           "During the weekend after working hours with consultant mode the cluster should scale down.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            6,
			currentHour:    19,
			consultantMode: true,
			expectedCap:    0,
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			cap := determineNewCapacity(test.startTime, test.endTime, test.cap, test.day, test.currentHour, test.consultantMode)
			if cap != test.expectedCap {
				t.Errorf("expected %d, got %d", test.expectedCap, cap)
			}

		})
	}

}
