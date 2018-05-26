package jailbuilder

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/briandowns/sky-island/utils"
	"github.com/mholt/archiver"
	"golang.org/x/sync/errgroup"
)

// basePackages contains the base packages
// necessary to install FreeBSD
var basePackages = []string{
	"base.txz",
	"lib32.txz",
	"ports.txz",
}

const rcConf = "/etc/rc.conf"
const sysDownloadURL = "http://ftp.freebsd.org/pub/FreeBSD/releases/amd64/amd64/%s/%s"

// Opts holds the options given
// when creating a builder
type Opts struct {
	BaseDir string
	Release string
	Dataset string
}

// Validate validates that the given opts are valid
func (o *Opts) Validate() error {
	switch {
	case o.BaseDir == "":
		return errors.New("missing base directory")
	case o.Release == "":
		return errors.New("missing release")
	case o.Dataset == "":
		return errors.New("missing dataset")
	default:
		return nil
	}
}

// Builder
type Builder struct {
	wrapper Wrapper
	opts    *Opts
}

// New creates a new value of type Builder pointer
func New(opts *Opts) (*Builder, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return &Builder{
		wrapper: &Wrap{},
		opts:    opts,
	}, nil
}

// DownloadBaseSystem
func (b *Builder) DownloadBaseSystem() error {
	if !utils.Exists("/tmp/" + b.opts.Release) {
		if err := os.Mkdir("/tmp/"+b.opts.Release, os.ModePerm); err != nil {
			return err
		}
	}
	var g errgroup.Group
	for _, p := range basePackages {
		file := "/tmp/" + b.opts.Release + "/" + p
		if utils.Exists(file) {
			continue
		}
		pkg := p
		pkgFile := file
		g.Go(func() error {
			out, err := os.Create(pkgFile)
			if err != nil {
				return err
			}
			defer out.Close()
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(sysDownloadURL, b.opts.Release, pkg), nil)
			if err != nil {
				return err
			}
			hc := &http.Client{
				Timeout: time.Second * 300,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			res, err := hc.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()
			_, err = io.Copy(out, res.Body)
			return err
		})
	}
	return g.Wait()
}

// ExtractBasePkgs
func (b *Builder) ExtractBasePkgs() error {
	fullPath := b.opts.BaseDir + "/releases/" + b.opts.Release
	for _, p := range basePackages {
		if err := archiver.TarXZ.Open("/tmp/"+b.opts.Release+"/"+p, fullPath); err != nil {
			return err
		}
	}
	return nil
}

// UpdateBaseJail
func (b *Builder) UpdateBaseJail() error {
	path := b.opts.BaseDir + "/releases/" + b.opts.Release
	cmd := exec.Command(
		"env", "UNAME_r="+b.opts.Release,
		"freebsd-update",
		"-b",
		"--not-running-from-cron",
		path,
		"fetch",
		"install",
	)
	env := os.Environ()
	env = append(env, "UNAME_r="+b.opts.Release)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// BaseJailSysConf
func (b *Builder) BaseJailSysConf() error {
	return nil
}

// SetLocaltime
func (b *Builder) SetLocaltime() error {
	in, err := os.Open("/etc/localtime")
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(b.opts.BaseDir + "/releases/" + b.opts.Release + "/etc/localtime")
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// SetResolveConf
func (b *Builder) SetResolvConf(nameservers []string) error {
	if nameservers != nil {
		out, err := os.Create(b.opts.BaseDir + "/releases/" + b.opts.Release + "/etc/resolv.conf")
		if err != nil {
			return err
		}
		defer out.Close()
		for _, i := range nameservers {
			out.Write([]byte("nameserver " + i + "\n"))
		}
		return nil
	}
	in, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(b.opts.BaseDir + "/releases/" + b.opts.Release + "/etc/resolv.conf")
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ConfigureJailHostname sets the hostname of the jail
// to the name of the jail
func (b *Builder) ConfigureJailHostname(name string) error {
	rcconf, err := os.Open(b.opts.BaseDir + "/" + name + "/etc/rc.conf")
	if err != nil {
		return err
	}
	defer rcconf.Close()
	if _, err := rcconf.Write([]byte("hostname=" + name)); err != nil {
		return err
	}
	return nil
}

// setBaseJailConf configures the jail to have the same resolv.conf
// and localtime as the host system
func (b *Builder) SetBaseJailConf() error {
	if err := b.SetResolvConf([]string{}); err != nil {
		return err
	}
	return nil
}

// Initialize
func (b *Builder) Initialize() error {
	if err := b.CreateZFSDataset(); err != nil {
		return err
	}
	if err := b.DownloadBaseSystem(); err != nil {
		return err
	}
	if err := b.ExtractBasePkgs(); err != nil {
		return err
	}
	if err := b.UpdateBaseJail(); err != nil {
		return err
	}
	if err := b.SetBaseJailConf(); err != nil {
		return err
	}
	if err := b.CreateZFSSnapshot(); err != nil {
		return err
	}
	return b.CreateJail("base")
}

// CreateJail
func (b *Builder) CreateJail(name string) error {
	if err := b.CloneBaseToJail(name); err != nil {
		return err
	}
	f, err := os.Create(b.opts.BaseDir + "/" + name + rcConf)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte(fmt.Sprintf(`hostname="%s"`, name))); err != nil {
		return err
	}
	return nil
}
