import { createRouter, createWebHashHistory } from "vue-router";
import BackupView from "./views/BackupView.vue";
import DashboardView from "./views/DashboardView.vue";
import DeepView from "./views/DeepView.vue";
import EnableView from "./views/EnableView.vue";
import PersistenceView from "./views/PersistenceView.vue";
import PrimitivesView from "./views/PrimitivesView.vue";
import ReportsView from "./views/ReportsView.vue";
import SettingsView from "./views/SettingsView.vue";

declare module "vue-router" {
  interface RouteMeta {
    title: string;
    description: string;
  }
}

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/",
      name: "dashboard",
      component: DashboardView,
      meta: {
        title: "设备预检",
        description: "只读检查：识别型号与固件、探测端口，可选验证密码 · 不会修改设备",
      },
    },
    {
      path: "/enable",
      name: "enable",
      component: EnableView,
      meta: {
        title: "开启 SSH",
        description: "临时开启 SSH 并验证 root 登录 · NVRAM 开关与 root 密码不会自动恢复",
      },
    },
    {
      path: "/persistence",
      name: "persistence",
      component: PersistenceView,
      meta: {
        title: "软固化与重启",
        description: "安装或移除工具管理的 auto_ssh 配置，并用一次重启验证它确实生效",
      },
    },
    {
      path: "/backup",
      name: "backup",
      component: BackupView,
      meta: {
        title: "分区备份",
        description: "逐分区读取 MTD，并校验长度与 SHA-256 · 备份不是原子快照",
      },
    },
    {
      path: "/deep",
      name: "deep",
      component: DeepView,
      meta: {
        title: "深度固化",
        description:
          "写入并擦除 MTD、修改 bdata，并重启三次 · 只在独立授权、可恢复的测试设备上使用",
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
        title: "连接设置",
        description: "目标地址、端口与 Profile，SSH 主机密钥，以及界面外观",
      },
    },
    {
      path: "/primitives",
      name: "primitives",
      component: PrimitivesView,
      meta: {
        title: "设计原语",
        description: "开发用展示页：先在两种主题下核对这些原语，再组装业务页面",
      },
    },
  ],
});
