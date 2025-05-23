package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
)

type Audio struct{}

var MAX_BUFFER = 1024

func (a *Audio) Encode(ctx context.Context, name string, data chan<- []byte, done chan bool) error {
	var err error
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i",
		fmt.Sprintf("./media/%s", name),
		"-ac",     // Set channel
		"2",       // Stereo
		"-ar",     // Set audio sampling rate.
		"48000",   // 48K audio sampling rate.
		"-c:a",    // Set audio codec
		"libopus", // Audio codec opus
		"-f",      // Force format option.
		"opus",    // Force format to opus.
		"-",       // Stream to stdout.
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err.Error())
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err.Error())
	}
	defer cmd.Wait()
	buff := make([]byte, 1024) // 1KB
	for {
		n, err := stdout.Read(buff)
		if err != nil {
			if errors.Is(err, io.EOF) {
				done <- true
				break
			}
			return err
		}
		if n > 0 {
			d := make([]byte, n)
			copy(d, buff[:n])
			fmt.Println(d)
			data <- d
		}
	}
	return nil
}
