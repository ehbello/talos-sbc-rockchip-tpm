// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-copy/copy"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
	"github.com/siderolabs/talos/pkg/machinery/overlay/adapter"
	"golang.org/x/sys/unix"
)

const (
	off   int64 = 512 * 64
	board       = "rockpi4c"
)

// List of boot files to copy
var bootFiles = []string{
	// https://github.com/u-boot/u-boot/blob/4de720e98d552dfda9278516bf788c4a73b3e56f/configs/rock-pi-4c-rk3399_defconfig#L7
	"dtb/rockchip/rk3399-rock-pi-4c.dtb",
	"dtb/overlays/*.dtbo",
	"boot.scr",
}

func main() {
	adapter.Execute(&rockPi4c{})
}

type rockPi4c struct{}

type rockPi4cExtraOptions struct{}

func (i *rockPi4c) GetOptions(extra rockPi4cExtraOptions) (overlay.Options, error) {
	return overlay.Options{
		Name: board,
		KernelArgs: []string{
			"console=tty0",
			"console=ttyS2,1500000n8",
			"sysctl.kernel.kexec_load_disabled=1",
			"talos.dashboard.disabled=1",
		},
		PartitionOptions: overlay.PartitionOptions{
			Offset: 2048 * 10,
		},
	}, nil
}

func (i *rockPi4c) Install(options overlay.InstallOptions[rockPi4cExtraOptions]) error {
	var f *os.File

	f, err := os.OpenFile(options.InstallDisk, os.O_RDWR|unix.O_CLOEXEC, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", options.InstallDisk, err)
	}

	defer f.Close() //nolint:errcheck

	uboot, err := os.ReadFile(filepath.Join(options.ArtifactsPath, "arm64/u-boot", board, "u-boot-rockchip.bin"))
	if err != nil {
		return err
	}

	if _, err = f.WriteAt(uboot, off); err != nil {
		return err
	}

	// NB: In the case that the block device is a loopback device, we sync here
	// to esure that the file is written before the loopback device is
	// unmounted.
	err = f.Sync()
	if err != nil {
		return err
	}

	for _, bootFile := range bootFiles {
		matches, err := filepath.Glob(filepath.Join(options.ArtifactsPath, "arm64/", bootFile))
		if err != nil {
			return err
		}

		for _, match := range matches {
			relPath, err := filepath.Rel(filepath.Join(options.ArtifactsPath, "arm64/"), match)
			if err != nil {
				return err
			}

			dst := filepath.Join(options.MountPrefix, "/boot/EFI/", relPath)

			err = os.MkdirAll(filepath.Dir(dst), 0o600)
			if err != nil {
				return err
			}

			err = copy.File(match, dst)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
