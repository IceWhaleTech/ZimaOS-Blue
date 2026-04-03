//go:build windows

package main

import (
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const rpcEChangedMode = 0x80010106

func openBrowserURL(rawURL string) error {
	shouldUninit, err := initializeShellCOM()
	if err != nil {
		return err
	}
	if shouldUninit {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return fmt.Errorf("create Shell.Application COM object: %w", err)
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query Shell.Application IDispatch: %w", err)
	}
	defer shell.Release()

	if _, err := oleutil.CallMethod(shell, "ShellExecute", rawURL, "", "", "open", 1); err != nil {
		return fmt.Errorf("call ShellExecute: %w", err)
	}
	return nil
}

func initializeShellCOM() (bool, error) {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err == nil {
		return true, nil
	}
	if oleErr, ok := err.(*ole.OleError); ok && oleErr.Code() == rpcEChangedMode {
		return false, nil
	}
	return false, fmt.Errorf("initialize COM apartment: %w", err)
}
