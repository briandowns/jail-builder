package jailbuilder

import "fmt"

// CreateZFSDataset
func (b *Builder) CreateZFSDataset() error {
	dataset := b.opts.Dataset + "/jails/releases/" + b.opts.Release
	out, err := b.wrapper.CombinedOutput("zfs", "create", "-p", dataset)
	fmt.Println(string(out))
	return err
}

// CreateSnapshot creates a ZFS snapshot of the base jail
func (b *Builder) CreateZFSSnapshot() error {
	_, err := b.wrapper.Output("zfs", "snapshot", b.opts.Dataset+"/jails/releases/"+b.opts.Release+"@p1")
	return err
}

// CloneBaseToJail does a ZFS clone from the base jail to the new jail
func (b *Builder) CloneBaseToJail(jname string) error {
	base := "/" + b.opts.Dataset + "/jails/releases/" + b.opts.Release + "@p1"
	dataset := "/zroot/jails/" + jname
	_, err := b.wrapper.Output("zfs", "clone", base, dataset)
	return err
}

// CreateBaseJailDataset creates a Dataset and mounts it for the base jail
func (b *Builder) CreateBaseJailDataset() error {
	_, err := b.wrapper.Output("zfs", "create", "-o", "mountpoint="+b.opts.BaseDir, "zroot", "")
	return err
}
