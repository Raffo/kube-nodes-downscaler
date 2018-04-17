package main

import (
	"fmt"
	"testing"
	"time"

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
		day            time.Weekday
		currentHour    int
		consultantMode bool
		expectedCap    int
	}{
		{
			name:           "During working hours in the mid of the week the cluster should keep its capacity.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            time.Wednesday,
			currentHour:    12,
			consultantMode: false,
			expectedCap:    2,
		},
		{
			name:           "After working hours in the mid of the week the cluster should scale down.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            time.Wednesday,
			currentHour:    19,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "After working hours in the mid of the week the cluster should stay scaled down.",
			startTime:      7,
			endTime:        16,
			cap:            0,
			day:            time.Thursday,
			currentHour:    2,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "Before working hours I should scale down!",
			startTime:      11,
			endTime:        16,
			cap:            2,
			day:            time.Thursday,
			currentHour:    8,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "During the weekend during the day, the cluster should stay scaled down.",
			startTime:      9,
			endTime:        18,
			cap:            0,
			day:            time.Saturday,
			currentHour:    19,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "During the weekend with consultant mode the cluster should scale up.",
			startTime:      9,
			endTime:        18,
			cap:            0,
			day:            time.Saturday,
			currentHour:    10,
			consultantMode: true,
			expectedCap:    2,
		},
		{
			name:           "During the weekend after working hours with consultant mode the cluster should scale down.",
			startTime:      9,
			endTime:        18,
			cap:            2,
			day:            time.Saturday,
			currentHour:    19,
			consultantMode: true,
			expectedCap:    0,
		},
		{
			name:           "On sunday I should stay off if consuntalt mode is not enabled.",
			startTime:      7,
			endTime:        17,
			cap:            2,
			day:            time.Sunday,
			currentHour:    16,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "The start time is exactly at the same time I specified (expected comparison).",
			startTime:      7,
			endTime:        17,
			cap:            0,
			day:            time.Monday,
			currentHour:    7,
			consultantMode: false,
			expectedCap:    2,
		},
		{
			name:           "The end time is exactly at the same time I specified (expected comparison).",
			startTime:      6,
			endTime:        17,
			cap:            2,
			day:            time.Monday,
			currentHour:    7,
			consultantMode: false,
			expectedCap:    2,
		},
		{
			name:           "Start time equal to end time is a valid choice, out of hours.",
			startTime:      17,
			endTime:        17,
			cap:            2,
			day:            time.Monday,
			currentHour:    7,
			consultantMode: false,
			expectedCap:    0,
		},
		{
			name:           "Start time equal to end time is a valid choice, at that time.",
			startTime:      17,
			endTime:        17,
			cap:            2,
			day:            time.Monday,
			currentHour:    17,
			consultantMode: false,
			expectedCap:    2,
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

func TestValidateParams(tt *testing.T) {

	for _, test := range []struct {
		name        string
		startTime   int
		endTime     int
		expectedErr bool
	}{
		{
			name:        "Normal working ours are valid.",
			startTime:   9,
			endTime:     18,
			expectedErr: false,
		},
		{
			name:        "Negative start is invalid.",
			startTime:   -1,
			endTime:     18,
			expectedErr: true,
		},
		{
			name:        "Negative end is invalid.",
			startTime:   0,
			endTime:     -1,
			expectedErr: true,
		},
		{
			name:        "0 is not a valid start.",
			startTime:   0,
			endTime:     18,
			expectedErr: true,
		},
		{
			name:        "24 as end is valid.",
			startTime:   1,
			endTime:     24,
			expectedErr: false,
		},
		{
			name:        "more than 24 as end is invalid.",
			startTime:   1,
			endTime:     25,
			expectedErr: true,
		},
	} {

		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			err := validateParams(test.startTime, test.endTime)
			if (err != nil) != test.expectedErr {
				t.Errorf("expected %v, got %v", test.expectedErr, err != nil)
			}
		})
	}
}
