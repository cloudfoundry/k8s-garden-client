package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <tar_path> <username> <target_path>\n", os.Args[0])
		os.Exit(1)
	}

	tarPath := os.Args[1]
	username := os.Args[2]
	targetPath := os.Args[3]

	uid, gid, err := getUserIdAndGroupId(username)
	if err != nil {
		exitWithError(fmt.Errorf("failed to get user id and group id: %v\n", err))
	}

	if err := mkdirPAs(targetPath, uid, gid); err != nil {
		exitWithError(fmt.Errorf("failed to chown directory %q: %w", targetPath, err))
	}

	if err := unix.Setgid(gid); err != nil {
		exitWithError(fmt.Errorf("failed to set gid: %w", err))
	}

	if err := unix.Setuid(uid); err != nil {
		exitWithError(fmt.Errorf("failed to set uid: %w", err))
	}

	if execerr := unix.Exec(tarPath, []string{tarPath, "-xf", "-", "-C", targetPath}, os.Environ()); execerr != nil {
		exitWithError(fmt.Errorf("failed to exec tar command: %w", execerr))
	}
}

func mkdirAs(dir string, uid, gid int) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil
		}

		return err
	}

	return os.Chown(dir, uid, gid)
}

func mkdirPAs(dir string, uid, gid int) error {
	if len(dir) > 1 && dir[len(dir)-1] == '/' {
		dir = dir[:len(dir)-1]
	}

	for i := 1; i < len(dir); i++ {
		if dir[i] == '/' {
			if err := mkdirAs(dir[:i], uid, gid); err != nil {
				return err
			}
		}
	}

	return mkdirAs(dir, uid, gid)
}

func getUserIdAndGroupId(username string) (int, int, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to lookup user %q: %w", username, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid %q: %w", u.Uid, err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid %q: %w", u.Gid, err)
	}

	return uid, gid, nil
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
