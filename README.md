# gitstatus

### To Install
`go install github.com/Jleagle/gitstatus`

### To Update
`gitstatus -update`

### To Run
`gitstatus` defaults to dry run, `gitstatus -pull` to pull repos.

### Code Directory
Scans `/users/user/code` by default, falls back to `os.Getenv("CODE_DIR")`, `gitstatus -d /codedir` to override.
