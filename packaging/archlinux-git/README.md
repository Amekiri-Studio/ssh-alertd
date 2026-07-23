# Arch Linux VCS package (ssh-alertd-git)

A `-git` package that builds ssh-alertd from the latest sources instead of a
release tarball. It `provides`/`conflicts` `ssh-alertd`, so it replaces the
stable AUR package.

## Build & install locally

```sh
cd packaging/archlinux-git
makepkg -si
```

`pkgver()` derives a version like `0.2.0.r5.gabc1234` (last tag + commits-since +
short hash) from `git describe`.

## Which branch it tracks

By default it clones the repository's default branch (`main`). For bleeding-edge
builds off `dev`, change the `source` line to:

```sh
source=("$pkgname::git+$url.git#branch=dev")
```

## Publishing to the AUR

Separate AUR package from `ssh-alertd`, named `ssh-alertd-git`:

```sh
git clone ssh://aur@aur.archlinux.org/ssh-alertd-git.git
cd ssh-alertd-git
cp ../ssh-alertd/packaging/archlinux-git/PKGBUILD .
cp ../ssh-alertd/packaging/archlinux-git/ssh-alertd.install .
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD ssh-alertd.install .SRCINFO
git commit -m "Initial import: ssh-alertd-git"
git push
```

For a VCS package the `.SRCINFO` `pkgver` is just a placeholder; it updates on
each user build. You only need to push again when the PKGBUILD itself changes.
