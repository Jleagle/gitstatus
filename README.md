# gitstatus

### Install

`brew install Jleagle/gitstatus/gitstatus`

Homebrew compiles it from source on your machine.

### Upgrading from the cask

Up to 2.3.0 gitstatus was a cask of prebuilt binaries. `brew upgrade` now
reports that cask as disabled; swap to the formula with

```
brew uninstall --cask gitstatus
brew install Jleagle/gitstatus/gitstatus
```

### Flags

```
  -a, --all             Show all Repos
  -c, --compact         Progress Only
  -d, --dir string      Directory
  -e, --expand          Full Paths
  -f, --filter string   Filter
      --flat            Ungrouped Output
  -m, --maxdepth int    Max Depth (default 2)
      --plain           Plain Output
  -p, --pull            Pull Repos
  -r, --readers int     Concurrent Status Reads (default 1 per CPU core)
  -s, --summary         Summary Line
  -w, --workers int     Concurrent Pulls (default 4 per CPU core)
```

### Environment variables

Every flag can also be set with an environment variable. Paste this into your
`~/.bashrc` or `~/.zshrc` and change what you need; the values shown are the
defaults.

```bash
# Directory to scan for repos
export GITSTATUS_DIR="$HOME/code"

# How many directory levels below GITSTATUS_DIR to look for repos
export GITSTATUS_MAXDEPTH=2

# Only include repos whose path contains one of these comma-separated terms,
# prefix a term with ! to exclude it instead, e.g. "work,!archive"
export GITSTATUS_FILTER=""

# Pull every repo that can be fast-forwarded
export GITSTATUS_PULL=false

# How many repos to read the status of at once, 0 means one per CPU core
export GITSTATUS_READERS=0

# How many pulls to run at once, 0 means four per CPU core
export GITSTATUS_WORKERS=0

# List clean repos too, not only the ones that need attention
export GITSTATUS_ALL=false

# One long list instead of grouping repos by status
export GITSTATUS_FLAT=false

# Print a summary line with totals and timing after the results
export GITSTATUS_SUMMARY=false

# Print full paths instead of shortening your home directory to ~
export GITSTATUS_EXPAND=false

# Plain uncoloured output, one line per repo, as used when piping
export GITSTATUS_PLAIN=false

# Only show the progress bar while running, not every repo
export GITSTATUS_COMPACT=false
```
