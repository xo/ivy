# The VMs of the review

The review compares ivy with Microsoft Office, with the XPS and GDI+ code of
Windows, and with Pages, Keynote and Numbers. These programs do not run on
Linux, so two VMs on the incus of the host of a maintainer run them. The
scripts here prepare the VMs, copy the corpus into them, run the exports,
and copy the results into the cache of the review.

| VM | System | Programs |
| --- | --- | --- |
| `ivy-win11` | Windows 11 Pro | Office Professional Plus 2024. Visio Professional 2024 is not installed yet. The XPS stack of WPF. GDI+ |
| `ivy-macos` | macOS 15 on Intel | Pages, Keynote and Numbers 12.1, with the signature of the App Store |

Publisher is not part of Office 2024, so a Publisher file has LibreOffice as
its only reference.

## How the VMs were made

On 2026-11-17 the VMs were copied from snapshots of the VMs of the topi
project on the same host, into the incus project `ivy`:

```sh
incus project create ivy -c features.images=false -c features.networks=false -c features.profiles=true
incus profile copy macos macos --project topi --target-project ivy
incus profile copy topi-windows11 windows11 --project topi --target-project ivy
incus copy topi-win11/clean-with-agent ivy-win11 --project topi --target-project ivy -p windows11
incus init --empty --vm ivy-macos --project ivy -p default -p macos -c image.os=macOS -c limits.cpu=4 -c limits.memory=8GiB -d root,size=80GiB
sudo cp --reflink=always /var/lib/incus/storage-pools/default/virtual-machines-snapshots/topi_topi-macos/setup-done/root.img /var/lib/incus/virtual-machines/ivy_ivy-macos/root.img
sudo cp --reflink=always /var/lib/incus/storage-pools/default/virtual-machines-snapshots/topi_topi-macos/setup-done/OVMF_VARS.4m.fd /var/lib/incus/virtual-machines/ivy_ivy-macos/OVMF_VARS.4m.fd
```

A copy of the macOS VM with `incus copy` did not start. Incus 7.6 puts a
qcow2 layer, `image_metadata/root.qcow2`, over the raw disk of a VM, and
passes its data file to QEMU as the fdset `incus_root-raw`. The QEMU
scriptlet of the profile `macos` looked for `incus_root_raw`, with an
underscore. It did not find the fdset, so QEMU opened the path of the data
file that the qcow2 file records, in a folder that only root can read. The
profile `macos` of the project ivy accepts both names:

```
data_file = find_fdset(node_name + '-raw') or find_fdset(node_name + '_raw')
```

The profile of the project topi is unchanged.

The copy also had the IDs of topi-macos: the serial number, the serial
number of the board (MLB), the system UUID and the ROM. These IDs are the
sample values of the public guides for OpenCore, the boot loader that starts
macOS in the VM. With them, the App Store refused the sign-in with an
unknown error. The IDs are in `EFI/OC/config.plist` on the EFI partition of
the VM. OpenCore Vault is `Secure`, so OpenCore boots only a `config.plist`
that a vault signs. The vault key is in `OpenCore.efi`, and the Secure Boot
key of the profile signs `OpenCore.efi`. ivy-macos got IDs of its own in
these steps:

1. Make a serial number and an MLB for iMacPro1,1 with `macserial` of the
   tree of the `orchard` container (project `default`), and a UUID with
   `uuidgen`. The ROM is the MAC address of `en0`.
2. Mount the EFI partition of ivy-macos, `/dev/disk0s1`, and copy its
   `EFI/OC` folder out. Write the new IDs into its `config.plist`.
3. In a temporary folder of the `orchard` container, outside its tree, put
   that folder with an unsigned `OpenCore.efi` from
   `audk/Build/OpenCorePkg/RELEASE_GCC/X64`. Run
   `audk/OpenCorePkg/Utilities/CreateVault/sign.command` on it, and sign
   `OpenCore.efi` with `sbsign` and `sb.key` and `sb.crt` of the tree, as
   `./orchard package --sign` does.
4. Copy `config.plist`, `OpenCore.efi`, `vault.plist` and `vault.sig` into
   `EFI/OC` of ivy-macos. Stop the VM fully and start it.

The EFI folder from before the change is in
`test/review/cache/vm/ivy-macos-efi-before-ids.tar`, which git ignores. The
tree of `orchard` and topi-macos are unchanged.

With the new IDs, the App Store still refused the sign-in. The scriptlet
moved the VirtIO network card of Incus to a USB card, so macOS did not
report `en0` as built in. macOS 15 has a driver for the VirtIO card,
`AppleVirtIONetwork`, so the scriptlet of the profile `macos` of the project
ivy leaves the card on PCI. It moves the card to USB only when the instance
sets `user.macos.usb_network` to `true`. The profile from before this change
is in `test/review/cache/vm/macos-profile-before-pci-network.yaml`. Before
the first start with the PCI card, `NetworkInterfaces.plist` in
`/Library/Preferences/SystemConfiguration` was moved to
`/Users/user/ivy/backup`, so that macOS names the new card `en0`.

The VM has not started with the PCI card yet. The host stopped before the
check. If macOS does not boot or has no network, set
`user.macos.usb_network` to `true` on ivy-macos, and the VM starts as
before.

The profile `windows11` gives 8 CPUs, 12 GiB, Secure Boot and a TPM. The
profile `macos` holds the QEMU scriptlet that macOS needs. The default
profile of the project connects the VMs to the bridge `incusbr0`. The user of
both VMs is `user`, which logs in by itself at boot.

The Windows copy was renamed IVY-WIN11. The clock of the host's RTC is local
time, so `RealTimeIsUniversal` is 0. `incus restart` does not restart
Windows, so the scripts shut it down from inside. Do not run a copy and its
topi VM at the same time.

Start one VM at a time, and first make sure that no other VM of the host
runs, in any project. On 2026-11-17 ivy-win11, ivy-macos and topi-macos ran
at the same time, and the host ran out of memory and stopped. Each Windows
VM takes 12 GiB, and each macOS VM takes 8 GiB.

After that stop, ivy-win11 was restored to its snapshot `base`: the copy of
`topi-win11/clean-with-agent`, renamed, without Office. The snapshot
`office-installed` holds the install of the Office Deployment Tool from
before.

Apple allows macOS in a VM only on Apple hardware. The maintainers decide
whether to run `ivy-macos` on another host.

## What the maintainer does once

1. Run `ivy-vm.sh start win`. Open the desktop of the Windows VM with
   `incus console ivy-win11 --project ivy --type=vga`.
2. In the desktop, install Office Professional Plus 2024 and Visio
   Professional 2024 in 64 bits, and activate them. `windows/office.xml`
   lists the parts of Office that the exports need.
3. Run `ivy-vm.sh setup win`. It sets Office for the user, so that the
   exports run without a question. If Office is not installed, it stops.
   `windows/setup.ps1 -Install` installs Office with the Office Deployment
   Tool and `windows/office.xml` instead, but does not activate it.
4. Run `ivy-vm.sh stop win` before you start the Mac VM.
5. Run `ivy-vm.sh setup mac`. If no Apple ID is signed in, it stops and says
   so. Open the desktop with `incus console ivy-macos --project ivy
   --type=vga`, sign in to the App Store, and run the setup again.
6. Run `ivy-vm.sh iwork` once while you watch the desktop of the Mac. macOS
   asks once for each of Pages, Keynote and Numbers whether `ivy-export` may
   control it. Click OK.

## How to make the references

The scripts of the exports are in `../windows/`, next to the review
command. For example:

```sh
test/cmd/review/vm/ivy-vm.sh windows test/cmd/review/windows/office2pdf.ps1 msoffice docx,doc,rtf,xlsx,xls,pptx,ppt -Passwords C:\\ivy\\files\\passwords.tsv
test/cmd/review/vm/ivy-vm.sh windows test/cmd/review/windows/xps2png.ps1 windows/xps xps,oxps
test/cmd/review/vm/ivy-vm.sh windows test/cmd/review/windows/metafile2png.ps1 windows/gdiplus emf,wmf,emz,wmz
test/cmd/review/vm/ivy-vm.sh iwork
test/cmd/review/vm/ivy-vm.sh stop win
```

Add `vsdx,vsd` to the extensions of the Office export when Visio is
installed. Without Visio, the export records the Visio files as failed, and
only `RETRY=1` sends them again.

On 2026-11-17 Office exported 81 of the 85 files. Office refuses the other
four, so LibreOffice is their only reference: Word cannot read
`libreoffice/rtf/uxtsorel.rtf` and `poi/docx/deep-table-cell.docx`,
PowerPoint 2024 does not open the PowerPoint 95 file `poi/ppt/ppt95.ppt`,
and PowerPoint fails to open `poi/pptx/a4-slides.pptx`. Pages, Keynote and
Numbers exported all four iWork files of the corpus.

`windows` copies the corpus files with the given extensions into
`C:\ivy\files`, runs the script in the desktop of the user, and copies the
output folder into `test/review/cache/NAME`. `iwork` does the same with
`ivy-macos`, into `test/review/cache/iwork`. Each run writes a TSV of its
results next to the files. The review command reads the PDFs and the PNGs
from the cache. `test/cmd/review/README.md` says how.

The outputs stay in the cache, which git ignores. The review only reads
them, and never starts a VM. `ivy-vm.sh` sends a file to a VM only when its
output is missing or its content changed: `test/review/cache/NAME/sources.tsv`
records the sha256 of each file and whether the export made its output. When
the cache is up to date, the command starts no VM. Set `FORCE=1` to export
every file again, or `RETRY=1` to try again the files that failed.
