package main

import (
	"fmt"
	"net/http"
	"time"
	"yandu/internal/model"
)

type healthClient interface {
	Get(string) (*http.Response, error)
}

func ensureAgentRunning(client healthClient, start func() error, requireRunning bool) error {
	probe := func() (bool, error) {
		response, err := client.Get("http://" + model.ManagementAddr + "/api/v1/health")
		if err != nil {
			return false, nil
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return false, fmt.Errorf("管理端口的健康检查返回 %d，请核对占用进程", response.StatusCode)
		}
		return true, nil
	}
	ready, err := probe()
	if err != nil || ready {
		return err
	}
	if requireRunning {
		return fmt.Errorf("本地 Agent 未运行，请先启动开发实例；本次不会启动后台进程或系统服务")
	}
	if err := start(); err != nil {
		return err
	}
	for i := 0; i < 80; i++ {
		ready, err = probe()
		if err != nil || ready {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("本地服务未启动，请查看状态目录中的 agent.log")
}
