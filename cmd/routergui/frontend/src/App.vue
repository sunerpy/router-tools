<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, provide, ref } from "vue";
import { useRoute } from "vue-router";
import { NConfigProvider, darkTheme } from "naive-ui";
import {
  Bot,
  FileClock,
  Fingerprint,
  Moon,
  Router as RouterIcon,
  Settings,
  Shapes,
  ShieldAlert,
  Sparkles,
  Sun,
  Wrench,
} from "lucide-vue-next";
import { useAgents } from "./composables/useAgents";
import { useGuide } from "./composables/useGuide";
import { useOperations } from "./composables/useOperations";
import type { ThemeController } from "./composables/useTheme";
import { createAPI, isPreview } from "./services/wails";
import { agentHeaderStatus, agentNavState } from "./services/agents";
import { guideStatus, latestOwnEntry, targetStatus } from "./services/summary";
import type { CatalogEntry } from "./services/api";
import { documentTokens, naiveOverrides } from "./styles/naiveTheme";
import HostKeyDialog from "./components/HostKeyDialog.vue";
import StatusBadge from "./components/StatusBadge.vue";

const theme = inject("theme") as ThemeController;
const api = createAPI();
const store = useOperations(api);
const guide = useGuide(api, store);
// Agent access is shared by its page and the sidebar's state word; Go pushes changes as agent:status.
const agents = useAgents(api);
let stopAgents = () => {};
onMounted(() => {
  stopAgents = agents.subscribe();
  void agents.load();
});
onUnmounted(() => stopAgents());
const catalog = ref<CatalogEntry[]>([]);
api
  .Catalog()
  .then((entries) => (catalog.value = entries ?? []))
  .catch(() => {});
provide("api", api);
provide("operations", store);
provide("guide", guide);
provide("agents", agents);
provide("catalog", catalog);
const showPrimitives = import.meta.env.DEV;
// The guide keeps its own run state and 「Agent 接入」 shows the agent operations; the other pages share the
// operations store.
const headerStatus = computed(() => {
  if (route.name === "guide") return guideStatus(guide.phase, guide.run, guide.error);
  if (route.name === "agents") return agentHeaderStatus(agents.status);
  return store.status;
});
const route = useRoute();

const naiveTheme = computed(() => (theme.resolved.value === "dark" ? darkTheme : null));
const overrides = computed(() => {
  void theme.resolved.value;
  return naiveOverrides(documentTokens());
});
const themeLabel = computed(() =>
  theme.resolved.value === "dark" ? "切换到浅色主题（白瓷）" : "切换到深色主题（石墨）",
);

const navigation = computed(() => [
  {
    label: "设备",
    items: [
      { to: "/", label: "向导", icon: Sparkles },
      { to: "/tools", label: "维护", icon: Wrench },
    ],
  },
  { label: "高风险", items: [{ to: "/deep", label: "深度固化", icon: ShieldAlert, risk: true }] },
  {
    label: "集成",
    items: [{ to: "/agents", label: "Agent 接入", icon: Bot, state: agentNavState(agents.status) }],
  },
  {
    label: "记录",
    items: [
      { to: "/reports", label: "运行报告", icon: FileClock, count: store.history.length },
      { to: "/settings", label: "设置", icon: Settings },
    ],
  },
]);

// The sidebar summarizes the window's latest detection: the guide's plan, or a preflight from 维护. An agent's
// preflight of another router must not read as a changed target.
const latestDetection = computed(() => {
  if (guide.plan) return guide.plan;
  return store.operation === "Doctor" && store.report
    ? store.report
    : (latestOwnEntry(store.history, "doctor")?.report as any);
});
const target = computed(() => targetStatus(latestDetection.value, store.target.host));
const hostKeyLabel = computed(() => {
  if (store.hostKey.PinSHA256) return "已固定主机密钥指纹";
  if (store.hostKey.AcceptNew) return "自动接受未知主机密钥";
  return "按 known_hosts 校验主机密钥";
});
</script>

<template>
  <NConfigProvider abstract :theme="naiveTheme" :theme-overrides="overrides">
    <div class="app-shell">
      <aside class="app-sidebar" aria-label="主导航">
        <div class="app-brand">
          <span class="app-brand__mark" aria-hidden="true"><RouterIcon :size="18" /></span>
          <strong>Router Tools</strong>
        </div>
        <nav class="app-nav">
          <section v-for="group in navigation" :key="group.label" class="app-nav__group">
            <p class="app-nav__heading">{{ group.label }}</p>
            <RouterLink
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="app-nav__link"
              :class="{ 'app-nav__link--risk': (item as any).risk }"
              active-class="app-nav__link--active"
            >
              <component :is="item.icon" :size="18" aria-hidden="true" />
              <span>{{ item.label }}</span>
              <span v-if="(item as any).count" class="app-nav__count mono">{{
                (item as any).count
              }}</span>
              <span v-if="(item as any).state" class="app-nav__count">{{
                (item as any).state
              }}</span>
            </RouterLink>
          </section>
        </nav>
        <section class="app-target" aria-label="当前目标">
          <p class="app-target__row">
            <span class="lamp" :class="`lamp--${target.tone}`" aria-hidden="true"></span>
            <strong class="mono app-target__host">{{ store.target.host || "未设置地址" }}</strong>
          </p>
          <p class="app-target__line">{{ target.label }}</p>
          <p v-if="target.detail" class="app-target__line">{{ target.detail }}</p>
          <p class="app-target__line app-target__row">
            <Fingerprint :size="14" aria-hidden="true" />{{ hostKeyLabel }}
          </p>
        </section>
        <div class="app-sidebar__buttons">
          <button
            type="button"
            class="app-square"
            :aria-label="themeLabel"
            :title="themeLabel"
            @click="theme.toggle()"
          >
            <Sun v-if="theme.resolved.value === 'dark'" :size="18" aria-hidden="true" />
            <Moon v-else :size="18" aria-hidden="true" />
          </button>
          <RouterLink
            v-if="showPrimitives"
            to="/primitives"
            class="app-square"
            aria-label="设计原语"
            title="设计原语"
          >
            <Shapes :size="18" aria-hidden="true" />
          </RouterLink>
        </div>
      </aside>
      <div class="app-workspace">
        <header class="app-header">
          <div class="app-header__text">
            <h1>{{ route.meta.title }}</h1>
            <p>{{ route.meta.description }}</p>
            <p v-if="isPreview" class="app-preview">浏览器预览 · 模拟数据，不连接路由器</p>
          </div>
          <StatusBadge :status="headerStatus" />
        </header>
        <main class="operation-content">
          <RouterView />
        </main>
      </div>
      <HostKeyDialog />
    </div>
  </NConfigProvider>
</template>

<style scoped>
.app-shell {
  display: grid;
  grid-template-columns: var(--rt-sidebar-width) minmax(0, 1fr);
  gap: var(--rt-shell-gap);
  width: 100%;
  height: 100%;
  padding: var(--rt-shell-gap);
}
.app-sidebar {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto auto;
  gap: var(--rt-space-3);
  min-height: 0;
  padding: var(--rt-space-3);
  background: var(--rt-sidebar);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-panel);
}
.app-brand {
  display: flex;
  align-items: center;
  gap: var(--rt-space-3);
  min-height: var(--rt-brand-height);
  padding: 0 var(--rt-space-2);
}
.app-brand__mark {
  display: grid;
  place-items: center;
  width: var(--rt-chip-size);
  height: var(--rt-chip-size);
  border-radius: var(--rt-r-panel);
  color: var(--rt-primary-fg);
  background: var(--rt-primary);
}
.app-brand strong {
  font-size: var(--rt-text-lg);
  font-weight: var(--rt-weight-strong);
}
.app-nav {
  display: grid;
  align-content: start;
  gap: var(--rt-space-2);
  min-height: 0;
  overflow: auto;
}
.app-nav__group {
  display: grid;
  gap: var(--rt-space-1);
}
.app-nav__heading {
  margin: var(--rt-space-2) var(--rt-space-3) 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.app-nav__link {
  display: flex;
  align-items: center;
  gap: var(--rt-space-3);
  min-height: var(--rt-nav-row);
  padding: 0 var(--rt-space-3);
  color: var(--rt-fg);
  border-radius: var(--rt-r-control);
  text-decoration: none;
}
@media (pointer: coarse) {
  .app-nav__link {
    min-height: var(--rt-touch-target);
  }
}
.app-nav__link:hover {
  background: var(--rt-hover);
}
.app-nav__link:active {
  background: var(--rt-pressed);
}
.app-nav__link--active {
  background: var(--rt-nav-active);
  font-weight: var(--rt-weight-strong);
}
.app-nav__link--active svg {
  color: var(--rt-accent-text);
}
.app-nav__link--risk svg {
  color: var(--rt-danger-text);
}
.app-nav__count {
  margin-left: auto;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
  font-weight: var(--rt-weight-regular);
}
.app-target {
  display: grid;
  gap: var(--rt-space-1);
  padding: var(--rt-space-3);
  background: var(--rt-surface);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-panel);
  font-size: var(--rt-text-sm);
}
.app-target p {
  margin: 0;
}
.app-target__row {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  min-width: 0;
}
.app-target__host {
  min-width: 0;
  overflow-wrap: anywhere;
}
.app-target__line {
  color: var(--rt-fg-muted);
  overflow-wrap: anywhere;
}
.app-sidebar__buttons {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--rt-space-2);
}
.app-square {
  display: grid;
  place-items: center;
  min-height: var(--rt-touch-target);
  color: var(--rt-fg);
  background: var(--rt-field);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.app-square:hover {
  background: var(--rt-hover);
}
.app-square.router-link-active {
  background: var(--rt-nav-active);
}
.app-workspace {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--rt-space-3);
  min-width: 0;
  min-height: 0;
}
.app-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--rt-space-3) var(--rt-space-4);
  padding: var(--rt-space-1) var(--rt-space-2) 0;
}
.app-header__text {
  min-width: 0;
}
.app-header h1 {
  margin: 0;
  font-size: var(--rt-text-3xl);
  font-weight: var(--rt-weight-strong);
}
.app-header p {
  margin: var(--rt-space-1) 0 0;
  color: var(--rt-fg-muted);
  overflow-wrap: anywhere;
}
.app-header .app-preview {
  color: var(--rt-warning-text);
  font-size: var(--rt-text-xs);
}
.operation-content {
  /* The containing block for absolutely positioned descendants (for example visually hidden text),
     so they scroll with the page instead of growing the window's own scroll area. */
  position: relative;
  min-width: 0;
  min-height: 0;
  overflow: auto;
}
.operation-content > :deep(*) {
  width: min(100%, var(--rt-content-max));
  margin: 0 auto;
}
@media (max-width: 768px) {
  .app-shell {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
    gap: var(--rt-space-3);
    padding: var(--rt-space-3);
  }
  .app-sidebar {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: auto auto;
  }
  .app-nav {
    grid-column: 1 / -1;
    grid-row: 2;
    display: flex;
    flex-wrap: wrap;
    overflow: visible;
  }
  .app-nav__group {
    display: contents;
  }
  .app-nav__heading,
  .app-target {
    display: none;
  }
  .app-nav__link {
    flex: 0 1 auto;
  }
  .app-sidebar__buttons {
    grid-column: 2;
    grid-row: 1;
    grid-template-columns: repeat(3, var(--rt-touch-target));
  }
}
@media (max-width: 480px) {
  .app-header h1 {
    font-size: var(--rt-text-2xl);
  }
}
</style>
