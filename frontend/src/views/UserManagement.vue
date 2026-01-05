<template>
  <div class="user-management">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>用户管理</span>
          <el-button type="primary" @click="showAddUserDialog">
            <el-icon><Plus /></el-icon>添加用户
          </el-button>
        </div>
      </template>
      
      <el-table :data="users" stripe v-loading="loading">
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column prop="role" label="角色" width="120">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'danger'">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="last_login" label="最后登录" width="180">
          <template #default="{ row }">
            {{ row.last_login ? formatDate(row.last_login) : '从未登录' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="showEditUserDialog(row)">编辑</el-button>
            <el-button size="small" type="warning" @click="toggleUserStatus(row)">
              {{ row.enabled ? '禁用' : '启用' }}
            </el-button>
            <el-button size="small" type="danger" @click="deleteUser(row)" :disabled="row.username === currentUser?.username">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    
    <!-- 添加/编辑用户对话框 -->
    <el-dialog 
      v-model="showUserDialog" 
      :title="isEditing ? '编辑用户' : '添加用户'" 
      width="450px"
    >
      <el-form ref="userFormRef" :model="userForm" :rules="userRules" label-width="100px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="userForm.username" :disabled="isEditing" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码" prop="password" v-if="!isEditing">
          <el-input v-model="userForm.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword" v-if="isEditing">
          <el-input v-model="userForm.newPassword" type="password" show-password placeholder="留空则不修改" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="userForm.role" placeholder="请选择角色">
            <el-option label="管理员" value="admin" />
            <el-option label="操作员" value="operator" />
            <el-option label="观看者" value="viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUserDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSaveUser" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import axios from 'axios'
import { getUserInfo } from '../lib/auth'

interface User {
  id: string
  username: string
  role: string
  enabled: boolean
  created_at: string
  updated_at: string
  last_login?: string
}

const users = ref<User[]>([])
const loading = ref(false)
const saving = ref(false)
const showUserDialog = ref(false)
const isEditing = ref(false)
const userFormRef = ref<FormInstance>()
const currentUser = ref<User | null>(null)

const userForm = reactive({
  username: '',
  password: '',
  newPassword: '',
  role: 'viewer'
})

const userRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度3-20个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少6个字符', trigger: 'blur' }
  ],
  role: [
    { required: true, message: '请选择角色', trigger: 'change' }
  ]
}

const roleLabel = (role: string) => {
  const roleMap: Record<string, string> = {
    admin: '管理员',
    operator: '操作员',
    viewer: '观看者'
  }
  return roleMap[role] || role
}

const roleTagType = (role: string) => {
  const typeMap: Record<string, string> = {
    admin: 'danger',
    operator: 'warning',
    viewer: ''
  }
  return typeMap[role] || ''
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN')
}

const loadCurrentUser = () => {
  currentUser.value = getUserInfo()
}

const loadUsers = async () => {
  loading.value = true
  try {
    const response = await axios.get('/api/auth/users')
    if (response.data.success) {
      users.value = response.data.users || []
    } else {
      ElMessage.error(response.data.error || '加载用户列表失败')
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载用户列表失败')
  } finally {
    loading.value = false
  }
}

const showAddUserDialog = () => {
  isEditing.value = false
  userForm.username = ''
  userForm.password = ''
  userForm.newPassword = ''
  userForm.role = 'viewer'
  showUserDialog.value = true
}

const showEditUserDialog = (user: User) => {
  isEditing.value = true
  userForm.username = user.username
  userForm.password = ''
  userForm.newPassword = ''
  userForm.role = user.role
  showUserDialog.value = true
}

const handleSaveUser = async () => {
  if (!userFormRef.value) return
  
  // 编辑时密码不是必填
  if (isEditing.value) {
    const valid = await userFormRef.value.validateField(['username', 'role']).catch(() => false)
    if (!valid) return
  } else {
    const valid = await userFormRef.value.validate().catch(() => false)
    if (!valid) return
  }
  
  saving.value = true
  try {
    if (isEditing.value) {
      // 更新用户
      const updates: Record<string, any> = { role: userForm.role }
      if (userForm.newPassword) {
        updates.password = userForm.newPassword
      }
      
      const response = await axios.put(`/api/auth/users/update?username=${encodeURIComponent(userForm.username)}`, updates)
      if (response.data.success) {
        ElMessage.success('用户更新成功')
        showUserDialog.value = false
        loadUsers()
      } else {
        ElMessage.error(response.data.error || '更新失败')
      }
    } else {
      // 创建用户
      const response = await axios.post('/api/auth/users', {
        username: userForm.username,
        password: userForm.password,
        role: userForm.role
      })
      if (response.data.success) {
        ElMessage.success('用户创建成功')
        showUserDialog.value = false
        loadUsers()
      } else {
        ElMessage.error(response.data.error || '创建失败')
      }
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '操作失败')
  } finally {
    saving.value = false
  }
}

const toggleUserStatus = async (user: User) => {
  const action = user.enabled ? '禁用' : '启用'
  try {
    await ElMessageBox.confirm(`确定要${action}用户 "${user.username}" 吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    const response = await axios.put(`/api/auth/users/update?username=${encodeURIComponent(user.username)}`, {
      enabled: !user.enabled
    })
    
    if (response.data.success) {
      ElMessage.success(`${action}成功`)
      loadUsers()
    } else {
      ElMessage.error(response.data.error || `${action}失败`)
    }
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || `${action}失败`)
    }
  }
}

const deleteUser = async (user: User) => {
  try {
    await ElMessageBox.confirm(`确定要删除用户 "${user.username}" 吗？此操作不可恢复。`, '警告', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'error'
    })
    
    const response = await axios.delete(`/api/auth/users/delete?username=${encodeURIComponent(user.username)}`)
    
    if (response.data.success) {
      ElMessage.success('用户删除成功')
      loadUsers()
    } else {
      ElMessage.error(response.data.error || '删除失败')
    }
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '删除失败')
    }
  }
}

onMounted(() => {
  loadCurrentUser()
  loadUsers()
})
</script>

<style scoped>
.user-management {
  padding: 10px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.el-table {
  width: 100%;
}
</style>
