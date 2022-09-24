# 信令服务器需求
## 需求描述：在线情况下，实现一对一和1对多呼叫，从拨打到接通，上层可使用信令实现跟微信1对1、1对多呼叫一致的体验。
## 技术相关要求：
- websock实现长链接（ws对web环境友好），每个端链接后需要先发送登陆指令完成身份校验，确保身份安全
- 登陆成功后服务器主动推送其他设备在线情况，以及其他设备上下线通知，也支持批量查询其他设备在线情况。设备至少有以下状态：在线、离线、繁忙、通话中
- 实现一对一和1对多呼叫，需充分考虑超时、未接通等异常情况的处理
- 有个特别注意的是，设备是否占线，由服务器判定
- 支持扩展消息，应用可用该通道发送自定义的消息
- 需要提供客户端对接说明文档，包含流程图和消息各式、消息定义说明。
- 可多机横向扩展部署


# HooCha Communication Signals


## Command
* Connect via jwt (jwt get from self API)
* DisConnect
* Status
* Mute/UnMute
* Config(Retry, Timout)
* Call
* Accept
* Decline
* Ignore
* Message
* Broadcast
* Custom_Payload


## Client Status Enum
* OnCall 电话中
* NotFound 不存在
* Offline 用户离线
* UnStable 网络不稳定
* Unknown 未知


## Deployment
Running Via Docker 