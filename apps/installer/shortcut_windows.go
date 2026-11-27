package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// createShortcut 用 WScript.Shell 创建 .lnk。
// 走 IDispatch 而不是手写 IShellLink 虚表：少 200 行，行为完全一样。
func createShortcut(lnk, target, icon string) error {
	// COM 的 STA 以线程为界，而 goroutine 在调用间可能被调度到别的线程：不锁线程时
	// WScript.Shell 的 CreateShortcut 偶发 “Exception occurred”（CI 上有复现）。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// CoInitializeEx 返回 S_FALSE 表示本线程已经初始化过 COM —— 那不是失败。
	// 实测：把它当错误会让静默安装在建快捷方式这一步整体失败。
	_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("创建 WScript.Shell 失败：%w", err)
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("获取 IDispatch 接口失败：%w", err)
	}
	defer shell.Release()

	link, err := oleutil.CallMethod(shell, "CreateShortcut", lnk)
	if err != nil {
		return fmt.Errorf("创建快捷方式失败：%w", err)
	}
	disp := link.ToIDispatch()
	defer disp.Release()

	if _, err := oleutil.PutProperty(disp, "TargetPath", target); err != nil {
		return fmt.Errorf("设置快捷方式目标失败：%w", err)
	}
	if _, err := os.Stat(icon); err == nil {
		if _, err := oleutil.PutProperty(disp, "IconLocation", icon); err != nil {
			return fmt.Errorf("设置快捷方式图标失败：%w", err)
		}
	}
	if _, err := oleutil.CallMethod(disp, "Save"); err != nil {
		return fmt.Errorf("保存快捷方式失败：%w", err)
	}
	return nil
}

// launch 启动装好的主程序。不等待退出：安装器可以随即关闭。
func launch(target string) error {
	cmd := exec.Command(target)
	cmd.Dir = filepathDir(target)
	return cmd.Start()
}

func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\\' || p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
