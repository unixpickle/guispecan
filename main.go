package main

import (
	"bufio"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/unixpickle/gogui"
)

const WindowWidth = 600
const WindowHeight = 400
const SpectrumCount = 100

var SpectrumLock sync.Mutex
var Spectrums = [][]float64{}

var Canvas gogui.Canvas
var SigChan = make(chan os.Signal, 1)

func main() {
	go readSpectrum()
	gogui.RunOnMain(setup)
	gogui.Main(&gogui.AppInfo{Name: "Spectrum Analysis"})
}

func setup() {
	w, err := gogui.NewWindow(gogui.Rect{0, 0, WindowWidth, WindowHeight})
	if err != nil {
		panic(err)
	}
	Canvas, err = gogui.NewCanvas(gogui.Rect{0, 0, WindowWidth, WindowHeight})
	if err != nil {
		panic(err)
	}
	w.Add(Canvas)
	w.SetTitle("Spectrum Analysis")
	w.Center()
	w.Show()
	w.SetCloseHandler(func() {
		SigChan <- nil
	})
	Canvas.SetDrawHandler(drawSpectrum)
}

func drawSpectrum(ctx gogui.DrawContext) {
	SpectrumLock.Lock()
	defer SpectrumLock.Unlock()

	for idx, spectrum := range Spectrums {
		barWidth := WindowWidth / float64(len(spectrum))
		opacity := float64(idx+1) / float64(len(Spectrums))
		ctx.SetStroke(gogui.Color{0x65 / 255.0, 0xbc / 255.0, 0xd4 / 255.0, opacity})
		ctx.BeginPath()
		for i, magnitude := range spectrum {
			height := WindowHeight * magnitude
			top := WindowHeight - height
			if i == 0 {
				ctx.MoveTo(float64(i)*barWidth, top)
			} else {
				ctx.LineTo(float64(i)*barWidth, top)
			}
		}
		ctx.StrokePath()
	}
}

func readSpectrum() {
	path, err := exec.LookPath("ubertooth-specan")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(path, "-g")

	go func() {
		signal.Notify(SigChan, syscall.SIGINT, syscall.SIGTERM)
		<-SigChan
		syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
	}()

	incoming, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	r := bufio.NewReader(incoming)
	curSpec := []float64{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}

		line = line[:len(line)-1]

		if len(line) == 0 {
			SpectrumLock.Lock()
			if len(Spectrums) == SpectrumCount {
				copy(Spectrums[:], Spectrums[1:])
				Spectrums = Spectrums[:len(Spectrums)-1]
			}
			Spectrums = append(Spectrums, curSpec)
			SpectrumLock.Unlock()
			curSpec = []float64{}
			gogui.RunOnMain(func() {
				Canvas.NeedsUpdate()
			})
			continue
		}

		comps := strings.Split(line, " ")
		if len(comps) != 2 {
			panic("unexpected output: " + line)
		}
		rssi, _ := strconv.Atoi(comps[1])
		curSpec = append(curSpec, math.Pow(10, float64(-rssi)/10))
	}

	if err := cmd.Wait(); err != nil {
		panic("ubertooth-specan error: " + err.Error())
	}
	os.Exit(0)
}
