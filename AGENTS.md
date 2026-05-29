# AGENTS.md

1. Admin UI 使用 shadcn 时，如果官方有组件，必须通过 `npx shadcn@latest add <component>` 引入官方组件，不手写替代组件。
2. Admin UI 需要扩展 shadcn 官方组件能力时,必须新增业务包裹组件承载扩展,禁止直接修改官方组件源码。
3. 修改 Admin UI 布局前必须先搞清现有样式/组件的职责和复用范围；能通过删除、拆分、重命名旧抽象解决时，优先做减法，禁止只在原有补丁上继续叠补丁。
