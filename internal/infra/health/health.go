package health

type Status struct {
	Ready bool `json:"ready"`
}

func Check() Status {
	return Status{Ready: true}
}
