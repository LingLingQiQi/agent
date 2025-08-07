# CLAUDE.md
这个文件是用来描述 tools 目录的功能和详细设计
## 目录功能
这里是基于 eino 的 tool 组件实现工具的封装,以供大模型来调用.工具的类型支持 http,sse,lambda等.

### 代码设计
- **文件**: 一个文件代表一个 tool 的封装
- **返回格式**: 所有文件和 tool 的封装,返回的类型都一样, 都是返回 []tool.BaseTool
