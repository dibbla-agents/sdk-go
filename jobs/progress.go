package jobs

import (
	"fmt"
	"io"
	"strings"
)

// ProgressBar represents a terminal progress bar
type ProgressBar struct {
	total   int
	current int
	width   int
	writer  io.Writer
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, writer io.Writer) *ProgressBar {
	return &ProgressBar{
		total:  total,
		width:  50,
		writer: writer,
	}
}

// Update updates the progress bar
func (pb *ProgressBar) Update(current int, message string) {
	pb.current = current

	if pb.total == 0 {
		fmt.Fprintf(pb.writer, "\r⏳ %s: %d processed...", message, current)
		return
	}

	percent := float64(current) / float64(pb.total)
	filled := int(percent * float64(pb.width))

	bar := strings.Repeat("=", filled)
	if filled < pb.width {
		bar += ">"
		bar += strings.Repeat(" ", pb.width-filled-1)
	}

	fmt.Fprintf(pb.writer, "\r[%s] %3d%% %s", bar, int(percent*100), message)
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	fmt.Fprintln(pb.writer)
}
