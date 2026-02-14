package main

import (
	"github.com/indeedhat/dotenv"
	"github.com/indeedhat/icl"
)

const (
	envAditionalServices dotenv.String = "PID1_ADITIONAL_SERVICES"
)

type AditionalServices struct {
	Version  int       `icl:"version"`
	Services []Service `icl:"service"`
}

type Service struct {
	Name        string   `icl:".param"`
	Command     string   `icl:"command"`
	Args        []string `icl:"args"`
	AutoRestart bool     `icl:"auto_restart"`
	// Critical defines a critical service for the environment, if a Critical service exits the main
	// process will be shut down
	Critical bool `icl:"critical"`
	// CaptureOutput defines if stderr/stdout should be redirected to the main processes stdout/stderr
	CaptureOutput bool `icl:"capture_output"`
	// CapturePrefix defines if the captured/forwarded output should be prefixed with the command Name
	CapturePrefix bool `icl:"capture_prefix"`
}

func loadConfig() (*AditionalServices, error) {
	if envAditionalServices.Get() == "" {
		return nil, nil
	}

	var services AditionalServices
	if err := icl.UnMarshalFile(envAditionalServices.Get(), &services); err != nil {
		return nil, err
	}

	return &services, nil
}
