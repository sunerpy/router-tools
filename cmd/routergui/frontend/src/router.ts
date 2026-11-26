import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";
import AgentView from "./views/AgentView.vue";
import DeepView from "./views/DeepView.vue";
import GuideView from "./views/GuideView.vue";
import PrimitivesView from "./views/PrimitivesView.vue";
import ReportsView from "./views/ReportsView.vue";
import SettingsView from "./views/SettingsView.vue";
import ToolsView from "./views/ToolsView.vue";

declare module "vue-router" {
  interface RouteMeta {
    title: string;
    description: string;
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: "/",
    name: "guide",
    component: GuideView,
    meta: {
      title: "向导",
      description:
        "先只读检测路由器，再一次确认、只执行缺少的步骤：开启 SSH、备份、软固化与重启验证",
    },
  },
  {
    path: "/tools",
    name: "tools",
    component: ToolsView,
    meta: {
      title: "维护",
      description: "单独的维护操作：移除软固化、校验备份、验证 root 登录与原始预检，各自单独确认",
    },
  },
  {
    path: "/deep",
    name: "deep",
    component: DeepView,
    meta: {
      title: "深度固化",
      description:
        "写入并擦除 MTD、修改 bdata，并按 profile 重启多次 · 只在独立授权、可恢复的测试设备上使用",
    },
  },
  {
    path: "/agents",
    name: "agents",
    component: AgentView,
    meta: {
      title: "Agent 接入",
      description:
        "让 Claude Code、Codex 等 Agent 通过 MCP 使用检测、备份和引导流程 · 写入路由器需要你单独开启",
    },
  },
  {
    path: "/reports",
    name: "reports",
    component: ReportsView,
    meta: {
      title: "运行报告",
      description: "本次运行以来的操作报告，选择一条查看完整 JSON",
    },
  },
  {
    path: "/settings",
    name: "settings",
    component: SettingsView,
    meta: {
      title: "设置",
      description: "默认设备与地址、SSH 主机密钥、已保存的密码，以及界面外观",
    },
  },
  // The step-by-step pages of v1 are now steps of the guide.
  { path: "/enable", redirect: "/" },
  { path: "/persistence", redirect: "/" },
  { path: "/backup", redirect: "/" },
];

if (import.meta.env.DEV) {
  routes.push({
    path: "/primitives",
    name: "primitives",
    component: PrimitivesView,
    meta: {
      title: "设计原语",
      description: "开发用展示页：先在两种主题下核对这些原语，再组装业务页面",
    },
  });
}

export const router = createRouter({ history: createWebHashHistory(), routes });
