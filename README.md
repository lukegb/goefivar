# efiboot/efivar

_This is not an official Google product._

efiboot and efivar are small libraries which wrap `libefiboot` and `libefivar` (from https://github.com/rhboot/efivar)
in a Go-friendly manner.

# efibootedit

`efibootedit` is a simple Go program for manipulating the kernel parameters for installed Linux distributions, usually those using EFISTUB method of booting the kernel.

It requires that https://github.com/rhboot/efivar is installed.
