# makepopular 部署与 Fork 同步状态

## 检查时间

2026 年 8 月 25 日。

## 代码版本

当前 Fork：

```text
https://github.com/meeBillyoo/pluto-new-api
```

上游仓库：

```text
https://github.com/QuantumNous/new-api
```

两边的 `main` 分支当前指向相同提交：

```text
2d8e50bf36e94200b809dfb39e73624ec48b1e23
```

因此，Fork 当前没有相对于上游的独立提交，也没有需要合并的上游代码差异。

## 部署服务器上的额外修改

服务器部署目录中的基础 Git 提交仍然是上面的提交，但服务器工作区另外保留了两处未提交修改：

```text
relay/channel/openai/relay_responses.go
relay/helper/stream_scanner.go
```

这些修改用于兼容部分 Codex Responses 流式调用：

1. 收到 `response.completed` 或 `response.done` 后，将流标记为正常完成，避免客户端已经收到完整结果后被记录为 `client_gone`。
2. 正常结束后关闭上游响应时，不再额外记录 `http2: response body closed` 错误日志。

这两处修改不会改变客户端请求参数、模型名称、输入内容或 Responses API 格式，只影响中转站内部的流结束判断和日志状态。模型上游最多只能看到中转站在收到最终事件后更早结束响应连接，无法据此识别具体客户端。

## 当前决定

暂不把上述两处部署修改提交到 Fork，也不修改上游仓库。服务器继续保持当前已部署版本运行。

需要注意：以后如果重新拉取代码、清理工作区或重新部署，服务器上的未提交修改可能会被覆盖。届时如果再次出现正常请求被记录为 `client_gone`，需要重新应用这两处修改，或者再将其整理为正式提交。
