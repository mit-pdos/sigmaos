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
	StatusCode    Tstatus
	StatusMessage string
	StatusData    interface{}
}

func MakeStatus(code Tstatus) *Status {
	return &Status{code, "", nil}
}

func MakeStatusInfo(code Tstatus, msg string, data interface{}) *Status {
	return &Status{code, msg, data}
}

func MakeStatusErr(msg string, data interface{}) *Status {
	return &Status{StatusErr, msg, data}
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

func (s *Status) Msg() string {
	return s.StatusMessage
}

func (s *Status) Error() string {
	return s.StatusMessage
}

func (s *Status) Data() interface{} {
	return s.StatusData
}

func (s *Status) String() string {
	return fmt.Sprintf("&{ statuscode:%v msg:%v data:%v }", s.StatusCode, s.StatusMessage, s.StatusData)
}
