package proc

import (
	"fmt"
)

type Tstatus uint8

const (
	StatusOK Tstatus = iota + 1
	StatusEvicted
	StatusErr
)

func (status Tstatus) String() string {
	switch status {
	case StatusOK:
		return "OK"
	case StatusEvicted:
		return "EVICTED"
	case StatusErr:
		return "ERROR"
	default:
		return "unkown status"
	}
}

type Status struct {
	StatusCode Tstatus
	StatusInfo string
}

func MakeStatus(code Tstatus) *Status {
	return &Status{code, ""}
}

func MakeStatusErr(info string) *Status {
	return &Status{StatusErr, info}
}

func (s *Status) IsStatusOK() bool {
	return s.StatusCode == StatusOK
}

func (s *Status) IsStatusEvicted() bool {
	return s.StatusCode == StatusEvicted
}

func (s *Status) IsStatusErr() bool {
	return s.StatusCode == StatusErr
}

func (s *Status) Info() string {
	return s.StatusInfo
}

func (s *Status) String() string {
	return fmt.Sprintf("&{ statuscode:%v info:%v }", s.StatusCode, s.StatusInfo)
}
