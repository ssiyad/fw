package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type fBase interface {
	fPath() string
	fModTime() time.Time
}

type file struct {
	fs.FileInfo
	Path string
}

func (f *file) fPath() string {
	return f.Path
}

func (f *file) fModTime() time.Time {
	return f.FileInfo.ModTime()
}

type dir struct {
	fs.FileInfo
	Path  string
	Files []*file
}

func (d *dir) fPath() string {
	return d.Path
}

func (d *dir) fModTime() time.Time {
	return d.FileInfo.ModTime()
}

type command struct {
	cmd     *exec.Cmd
	e       []string
	s       string
	sharedB *bytes.Buffer
}

func (c *command) prepare() {
	c.sharedB.Reset()
	c.cmd = exec.Command(c.e[0], strings.Join(c.e[1:], " "), c.s)
	c.cmd.Stdout = c.sharedB
	c.cmd.Stderr = c.sharedB
}

func (c *command) fire() {
	c.prepare()
	err := c.cmd.Run()
	if err != nil {
		panic(err)
	}
	fmt.Println(c.sharedB.String())
}

func newCmd(e string, s string) *command {
	cmd := new(command)
	cmd.e = strings.Split(e, " ")
	cmd.s = s
	cmd.sharedB = &bytes.Buffer{}
	return cmd
}

type config struct {
	Interval int
	Command  *command
	Files    []*file
	Dirs     []*dir
}

func main() {
	c := initConfig()

	c.Command.fire()

	for {
		var flag bool
		for _, v := range c.Files {
			m, f := isModified(v)
			if m {
				v.FileInfo = f
				flag = true
				break
			}
		}
		if flag {
			c.Command.fire()
			continue
		}
		for _, v := range c.Dirs {
			m, f := isModified(v)
			if m {
				v.FileInfo = f
				v.Files = syncDir(v)
				flag = true
				break
			}
		}
		if flag {
			c.Command.fire()
			continue
		}
		for _, d := range c.Dirs {
			for _, f := range d.Files {
				m, i := isModified(f)
				if m {
					f.FileInfo = i
					flag = true
					break
				}
			}
		}
		if flag {
			c.Command.fire()
			continue
		}
		time.Sleep(time.Duration(c.Interval) * time.Second)
	}
}

func initConfig() *config {
	var intervalFlag int
	flag.IntVar(&intervalFlag, "interval", 1, "interval in seconds to check for updates")

	var execFlag string
	flag.StringVar(&execFlag, "exec", "sh -c", "exec to use instead of `sh -c`")

	flag.Parse()

	c := new(config)
	c.Interval = intervalFlag
	c.Command = newCmd(execFlag, flag.Arg(len(flag.Args())-1))

	for _, v := range flag.Args()[:len(flag.Args())-1] {
		f, err := os.Stat(v)
		if err != nil {
			panic(err)
		}
		fp, err := filepath.Abs(v)
		if err != nil {
			panic(err)
		}
		if f.IsDir() {
			d := new(dir)
			d.FileInfo = f
			d.Path = fp
			d.Files = syncDir(d)
			c.Dirs = append(c.Dirs, d)
		} else {
			c.Files = append(c.Files, &file{f, fp})
		}
	}

	return c
}

func isModified(f fBase) (bool, fs.FileInfo) {
	s, err := os.Stat(f.fPath())
	if err != nil {
		panic(err)
	}
	return !s.ModTime().Equal(f.fModTime()), s
}

func syncDir(d *dir) []*file {
	files, err := os.ReadDir(d.Path)
	if err != nil {
		panic(err)
	}
	var t []*file
	for _, f := range files {
		i, err := f.Info()
		if err != nil {
			panic(err)
		}
		filePath, err := filepath.Abs(filepath.Join(d.FileInfo.Name(), i.Name()))
		if err != nil {
			panic(err)
		}
		t = append(t, &file{i, filePath})
	}
	return t
}
