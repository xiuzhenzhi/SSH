package main

const diagnosticLimit = 4096

// Drain subprocess stderr without retaining an unbounded diagnostic stream.
type diagnosticBuffer struct{ data []byte }

func (b *diagnosticBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := diagnosticLimit - len(b.data); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *diagnosticBuffer) String() string { return string(b.data) }
