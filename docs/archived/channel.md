# Channel

除了内置的IM，CSGClaw也支持其他Channel：
- Feishu
- Matrix

## Feishu

当前版块阐述如何实现对Feishu渠道的支持。

### API层面

类似现有的IM路由：
- 创建用户：POST /api/v1/channels/feishu/users
- 获取用户列表：GET /api/v1/channels/feishu/users
- 创建房间：POST /api/v1/channels/feishu/rooms
- 获取房间列表：GET /api/v1/channels/feishu/rooms
- 添加成员：POST /api/v1/channels/feishu/rooms/<room_id>/members
- 获取成员列表：GET /api/v1/channels/feishu/rooms/<room_id>/members

### 核心业务代码

位于 /internal/channel/feishu.go

### CLI能力

- `csgclaw user`: 添加 `-channel` 可选参数，默认为csgclaw，就是当前内置的IM；如果指定为feishu，就走feishu模块的功能
- `csgclaw room`: 添加 `-channel` 可选参数，默认为csgclaw，就是当前内置的IM；如果指定为feishu，就走feishu模块的功能
