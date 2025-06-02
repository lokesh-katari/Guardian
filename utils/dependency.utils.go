package utils

import "os/exec"

func CheckFfmpeg() bool {
	cmd := exec.Command("which", "ffmpeg")
	err := cmd.Run()
	return err == nil
}

func CheckV4l2Ctl() bool {
	cmd := exec.Command("which", "v4l2-ctl")
	err := cmd.Run()
	return err == nil
}
