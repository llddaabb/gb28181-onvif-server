<template>
  <div class="app-container">
    <!-- 登录页面不显示布局 -->
    <template v-if="$route.path === '/login'">
      <router-view />
    </template>
    
    <!-- 主页面布局 -->
    <template v-else>
      <el-container>
        <el-header>
          <div class="header-left">
            <h1>CCTV管理系统</h1>
          </div>
          <div class="header-right">
            <el-dropdown @command="handleCommand">
              <span class="user-info">
                <el-icon><User /></el-icon>
                <span>{{ currentUser?.username || '未登录' }}</span>
                <span class="user-role">({{ roleLabel }})</span>
                <el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="isAdmin" command="users">
                    <el-icon><UserFilled /></el-icon>用户管理
                  </el-dropdown-item>
                  <el-dropdown-item command="password">
                    <el-icon><Key /></el-icon>修改密码
                  </el-dropdown-item>
                  <el-dropdown-item divided command="logout">
                    <el-icon><SwitchButton /></el-icon>退出登录
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
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
    </template>
    
    <!-- 修改密码对话框 -->
    <el-dialog v-model="showPasswordDialog" title="修改密码" width="400px">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="100px">
        <el-form-item label="原密码" prop="oldPassword">
          <el-input v-model="passwordForm.oldPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="passwordForm.newPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPasswordDialog = false">取消</el-button>
        <el-button type="primary" @click="handleChangePassword" :loading="changingPassword">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { 
  VideoCamera, Monitor, SwitchButton, Setting, VideoPlay, Platform, 
  FolderOpened, User, ArrowDown, UserFilled, Key 
} from '@element-plus/icons-vue'
import axios from 'axios'

interface UserInfo {
  id: string
  username: string
  role: string
  enabled: boolean
}

const router = useRouter()
const route = useRoute()
const activeMenu = ref('gb28181')
const currentUser = ref<UserInfo | null>(null)
const showPasswordDialog = ref(false)
const changingPassword = ref(false)
const passwordFormRef = ref<FormInstance>()

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule: any, value: string, callback: any) => {
  if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  oldPassword: [
    { required: true, message: '请输入原密码', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少6个字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const isAdmin = computed(() => currentUser.value?.role === 'admin')

const roleLabel = computed(() => {
  const roleMap: Record<string, string> = {
    admin: '管理员',
    operator: '操作员',
    viewer: '观看者'
  }
  return roleMap[currentUser.value?.role || ''] || '未知'
})

const handleMenuSelect = (key: string) => {
  activeMenu.value = key
  router.push(`/${key}`)
}

const handleCommand = (command: string) => {
  switch (command) {
    case 'users':
      router.push('/users')
      break
    case 'password':
      showPasswordDialog.value = true
      break
    case 'logout':
      handleLogout()
      break
  }
}

const handleLogout = async () => {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    // 调用登出接口
    try {
      await axios.post('/api/auth/logout')
    } catch (e) {
      // 忽略登出错误
    }
    
    // 清除认证信息
    localStorage.removeItem('auth_token')
    localStorage.removeItem('user_info')
    sessionStorage.removeItem('auth_token')
    sessionStorage.removeItem('user_info')
    
    ElMessage.success('已退出登录')
    router.push('/login')
  } catch (e) {
    // 用户取消
  }
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return
  
  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) return
  
  changingPassword.value = true
  try {
    const response = await axios.put('/api/auth/password', {
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword
    })
    
    if (response.data.success) {
      ElMessage.success('密码修改成功')
      showPasswordDialog.value = false
      passwordForm.oldPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
    } else {
      ElMessage.error(response.data.error || '修改失败')
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '修改失败')
  } finally {
    changingPassword.value = false
  }
}

const loadUserInfo = () => {
  const userInfoStr = localStorage.getItem('user_info') || sessionStorage.getItem('user_info')
  try {
    currentUser.value = userInfoStr ? JSON.parse(userInfoStr) : null
  } catch (e) {
    currentUser.value = null
  }
}

// 根据当前路由更新激活菜单
onMounted(() => {
  loadUserInfo()
  
  const currentPath = route.path
  if (currentPath === '/' || currentPath === '/login') {
    activeMenu.value = 'gb28181'
  } else {
    activeMenu.value = currentPath.replace('/', '')
  }
})

// 监听路由变化
router.afterEach((to) => {
  loadUserInfo()
  
  if (to.path === '/' || to.path === '/login') {
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
  justify-content: space-between;
  padding: 0 20px;
}

.header-left h1 {
  margin: 0;
  font-size: 20px;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  color: white;
  font-size: 14px;
}

.user-info .el-icon {
  margin-right: 5px;
}

.user-role {
  margin-left: 5px;
  opacity: 0.8;
  font-size: 12px;
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