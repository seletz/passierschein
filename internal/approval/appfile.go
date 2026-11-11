package approval

import (
	"fmt"
	"io/fs"
	"syscall"
)

// CheckAppFile checks the file of the approval app (D6, D40, G5 is a
// separate check). The file must be a regular file that the owner can
// execute, owned by the user uid or by root, with no write permission for
// group or others and with no setuid or setgid bit.
//
// The caller passes the information of the file after it followed a
// symbolic link. The check does not look at the directory of the file.
func CheckAppFile(fi fs.FileInfo, uid int) error {
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("the approval app is not a regular file")
	}
	m := fi.Mode()
	if m&(fs.ModeSetuid|fs.ModeSetgid) != 0 {
		return fmt.Errorf("the approval app has a setuid or setgid bit")
	}
	if m.Perm()&0o022 != 0 {
		return fmt.Errorf("the approval app is writable by group or others (mode %04o)", m.Perm())
	}
	if m.Perm()&0o100 == 0 {
		return fmt.Errorf("the owner cannot execute the approval app (mode %04o)", m.Perm())
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// Fail closed: without the owner we cannot judge the file.
		return fmt.Errorf("the owner of the approval app is unknown")
	}
	if int(st.Uid) != uid && st.Uid != 0 {
		return fmt.Errorf("the approval app belongs to user %d, not to the user or root", st.Uid)
	}
	return nil
}
