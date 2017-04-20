package patientheap

import (
	"container/heap"
)

type Patient struct {
	ID         string
	LastUpdate string
}

type PatientOutput struct {
	Patient
	Remaining int
}

type patientHeap []Patient

// implement heap.Interface

func (p patientHeap) Len() int            { return len(p) }
func (p patientHeap) Less(i, j int) bool  { return p[i].LastUpdate > p[j].LastUpdate }
func (p patientHeap) Swap(i, j int)       { p[i], p[j] = p[j], p[i] }
func (p *patientHeap) Push(x interface{}) { *p = append(*p, x.(Patient)) }
func (p *patientHeap) Pop() (x interface{}) {
	old := *p
	n := len(old)
	x = old[n-1]
	*p = old[0 : n-1]
	return x
}

func (p patientHeap) firstWithLength() (result PatientOutput) {
	if l := len(p); l > 0 {
		result = PatientOutput{p[0], l - 1}
	}
	return
}

// SortPatients takes a channel of Patients with an update timestamp and appeends them to a channel most recently changed patients first.
//
func SortPatients(done <-chan struct{}, patients <-chan Patient) <-chan PatientOutput {
	var sorted = make(chan PatientOutput, 0)
	var h = make(patientHeap, 0)
	var output chan<- PatientOutput = nil // output channel is nil while heap is empty

	go func() {
		defer close(sorted)
		for {
			select {
			case output <- h.firstWithLength():
				heap.Remove(&h, 0)
				if len(h) == 0 {
					output = nil
				}
			case patient, ok := <-patients:
				if !ok {
					patients = nil
				} else {
					heap.Push(&h, patient)
					output = sorted
				}
			case <-done:
				return
			}

			if patients == nil && output == nil {
				return
			}
		}
	}()

	return sorted
}
