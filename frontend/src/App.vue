<template>
  <div class="app-container">
    <el-container>
      <el-header>
        <h1>CCTV管理系统</h1>
      </el-header>
      <el-container>
        <el-aside width="200px">
          <el-menu
            :default-active="activeMenu"
            class="el-menu-vertical-demo"
            @select="handleMenuSelect"
          >
            <el-menu-item index="gb28181">
              <el-icon><VideoCamera /></el-icon>
              <span>GB28181设备</span>
            </el-menu-item>
            <el-menu-item index="onvif">
              <el-icon><Monitor /></el-icon>
              <span>ONVIF设备管理</span>
            </el-menu-item>
            <el-menu-item index="channel">
          <el-icon><SwitchButton /></el-icon>
          <span>通道管理</span>
        </el-menu-item>
        <el-menu-item index="stream">
          <el-icon><SwitchButton /></el-icon>
          <span>媒体流管理</span>
        </el-menu-item>
        <el-menu-item index="preview">
          <el-icon><VideoPlay /></el-icon>
          <span>多画面预览</span>
        </el-menu-item>
        <el-menu-item index="playback">
          <el-icon><SwitchButton /></el-icon>
          <span>录像回放</span>
        </el-menu-item>
        <el-menu-item index="storage">
          <el-icon><FolderOpened /></el-icon>
          <span>存储管理</span>
        </el-menu-item>
        <el-menu-item index="status">
          <el-icon><Monitor /></el-icon>
          <span>系统监控</span>
        </el-menu-item>
        <el-menu-item index="zlm">
          <el-icon><Platform /></el-icon>
          <span>ZLM媒体服务</span>
        </el-menu-item>
        <el-menu-item index="settings">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
          </el-menu>
        </el-aside>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { VideoCamera, Monitor, SwitchButton, Setting, Menu as IconMenu, Connection, VideoPlay, Platform } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const activeMenu = ref('gb28181')

const handleMenuSelect = (key: string) => {
  activeMenu.value = key
  router.push(`/${key}`)
}

// 根据当前路由更新激活菜单
onMounted(() => {
  const currentPath = route.path
  if (currentPath === '/') {
    activeMenu.value = 'gb28181'
  } else {
    activeMenu.value = currentPath.replace('/', '')
  }
})

// 监听路由变化
router.afterEach((to) => {
  if (to.path === '/') {
    activeMenu.value = 'gb28181'
  } else {
    activeMenu.value = to.path.replace('/', '')
  }
})
</script>

<style scoped>
.app-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.el-header {
  background-color: #1890ff;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.el-aside {
  background-color: #f0f2f5;
}

.el-menu-vertical-demo {
  height: 100%;
  border-right: none;
}

.el-main {
  padding: 20px;
  overflow: auto;
}
</style>