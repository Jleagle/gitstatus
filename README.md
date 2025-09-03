# gitstatus

### To Install

`go install github.com/Jleagle/gitstatus@latest`

### To Update

`gitstatus -update`

### To Run

`gitstatus` defaults to dry run, `gitstatus -pull` to pull repos.

### Code Directory

Scans `/users/user/code` by default, falls back to `os.Getenv("GITSTATUS_DIR")`, `gitstatus -dir /codedir` to override.

### ENV

```
GITSTATUS_DIR="/Users/jleagle/code"
GITSTATUS_FULL="true"
GITSTATUS_STALE="true"
```
