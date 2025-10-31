# gitstatus

### To Install

`brew install Jleagle/gitstatus/gitstatus`

### To Run

`gitstatus` defaults to dry run, `gitstatus -pull` to pull repos.

### Code Directory

Looks in `gitstatus -dir /dir`, then
`os.Getenv("GITSTATUS_DIR")`, then
`/users/user/code`

### ENVs

```
GITSTATUS_DIR="/Users/jleagle/code"
GITSTATUS_FULL="true"
```
