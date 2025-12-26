<template>
  <div class="storage-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>存储管理</span>
          <el-button type="primary" @click="showAddDiskDialog">添加磁盘</el-button>
        </div>
      </template>

      <!-- 存储统计 -->
      <el-row :gutter="20" class="stats-row">
        <el-col :span="6">
          <el-statistic title="磁盘总数" :value="stats.totalDisks || 0" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="在线磁盘" :value="stats.onlineDisks || 0" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="总容量" :value="formatSize(stats.totalSize)" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="已用" :value="`${stats.usedPercent?.toFixed(1) || 0}%`">
            <template #suffix>
              <el-progress 
                :percentage="stats.usedPercent || 0" 
                :color="getProgressColor(stats.usedPercent || 0)"
                :show-text="false"
                style="width: 100px; margin-left: 10px;"
              />
            </template>
          </el-statistic>
        </el-col>
      </el-row>

      <!-- 磁盘列表 -->
      <el-table :data="disks" style="width: 100%; margin-top: 20px;">
        <el-table-column prop="name" label="名称" width="150" />
        <el-table-column prop="mountPoint" label="挂载点" width="200" show-overflow-tooltip />
        <el-table-column label="容量" width="180">
          <template #default="scope">
            {{ formatSize(scope.row.usedSize) }} / {{ formatSize(scope.row.totalSize) }}
          </template>
        </el-table-column>
        <el-table-column label="使用率" width="200">
          <template #default="scope">
            <el-progress 
              :percentage="(scope.row.usedSize / scope.row.totalSize * 100) || 0" 
              :color="getProgressColor((scope.row.usedSize / scope.row.totalSize * 100) || 0)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">
              {{ getStatusText(scope.row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="80" />
        <el-table-column prop="enabled" label="启用" width="80">
          <template #default="scope">
            <el-switch v-model="scope.row.enabled" @change="updateDisk(scope.row)" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="scope">
            <el-button size="small" @click="editDisk(scope.row)">编辑</el-button>
            <el-button size="small" type="danger" @click="removeDisk(scope.row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 循环录制策略 -->
    <el-card style="margin-top: 20px;">
      <template #header>
        <div class="card-header">
          <span>循环录制策略</span>
        </div>
      </template>

      <el-form :model="recyclePolicy" label-width="180px">
        <el-form-item label="启用循环录制">
          <el-switch v-model="recyclePolicy.enabled" />
        </el-form-item>

        <el-form-item label="循环模式">
          <el-select v-model="recyclePolicy.mode" :disabled="!recyclePolicy.enabled">
            <el-option label="删除最老文件" value="oldest" />
            <el-option label="按时间删除" value="by_time" />
            <el-option label="按大小删除" value="by_size" />
            <el-option label="按数量删除" value="by_count" />
          </el-select>
        </el-form-item>

        <el-form-item label="保留天数" v-if="recyclePolicy.mode === 'by_time'">
          <el-input-number 
            v-model="recyclePolicy.keepDays" 
            :min="1" 
            :max="365"
            :disabled="!recyclePolicy.enabled"
          /> 天
        </el-form-item>

        <el-form-item label="保留容量" v-if="recyclePolicy.mode === 'by_size'">
          <el-input-number 
            v-model="recyclePolicy.keepSizeGB" 
            :min="1" 
            :max="10000"
            :disabled="!recyclePolicy.enabled"
          /> GB
        </el-form-item>

        <el-form-item label="保留文件数" v-if="recyclePolicy.mode === 'by_count'">
          <el-input-number 
            v-model="recyclePolicy.keepCount" 
            :min="1" 
            :max="10000"
            :disabled="!recyclePolicy.enabled"
          />
        </el-form-item>

        <el-form-item label="最小剩余空间">
          <el-input-number 
            v-model="recyclePolicy.minFreeSpacePercent" 
            :min="5" 
            :max="50"
            :disabled="!recyclePolicy.enabled"
          /> %
          <span style="margin-left: 10px; color: #999;">
            当剩余空间低于此值时触发循环删除
          </span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="saveRecyclePolicy">保存策略</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 添加/编辑磁盘对话框 -->
    <el-dialog 
      v-model="diskDialogVisible" 
      :title="isEdit ? '编辑磁盘' : '添加磁盘'" 
      width="600px"
    >
      <el-form :model="currentDisk" label-width="120px">
        <el-form-item label="磁盘ID" v-if="!isEdit">
          <el-input v-model="currentDisk.id" placeholder="disk_1" />
        </el-form-item>
        <el-form-item label="磁盘名称">
          <el-input v-model="currentDisk.name" placeholder="录像磁盘1" />
        </el-form-item>
        <el-form-item label="挂载点">
          <el-input v-model="currentDisk.mountPoint" placeholder="/mnt/recordings1" />
        </el-form-item>
        <el-form-item label="设备路径">
          <el-input v-model="currentDisk.devicePath" placeholder="/dev/sda1" />
        </el-form-item>
        <el-form-item label="文件系统">
          <el-input v-model="currentDisk.fileSystem" placeholder="ext4" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="currentDisk.priority" :min="0" :max="100" />
          <span style="margin-left: 10px; color: #999;">数字越小优先级越高</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="currentDisk.enabled" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="currentDisk.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="diskDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveDisk">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'

interface Disk {
  id: string
  name: string
  mountPoint: string
  totalSize: number
  usedSize: number
  freeSize: number
  status: string
  priority: number
  enabled: boolean
  devicePath?: string
  fileSystem?: string
  description?: string
}

interface RecyclePolicy {
  enabled: boolean
  mode: string
  keepDays: number
  keepSizeGB: number
  keepCount: number
  minFreeSpacePercent: number
}

const disks = ref<Disk[]>([])
const stats = ref<any>({})
const recyclePolicy = ref<RecyclePolicy>({
  enabled: true,
  mode: 'oldest',
  keepDays: 30,
  keepSizeGB: 100,
  keepCount: 1000,
  minFreeSpacePercent: 10
})

const diskDialogVisible = ref(false)
const isEdit = ref(false)
const currentDisk = ref<Disk>({
  id: '',
  name: '',
  mountPoint: '',
  totalSize: 0,
  usedSize: 0,
  freeSize: 0,
  status: 'online',
  priority: 0,
  enabled: true,
  devicePath: '',
  fileSystem: 'ext4',
  description: ''
})

const fetchDisks = async () => {
  try {
    const response = await axios.get('/api/storage/disks')
    if (response.data.success) {
      disks.value = response.data.disks || []
      stats.value = response.data.stats || {}
    }
  } catch (error) {
    ElMessage.error('获取磁盘列表失败')
    console.error('获取磁盘列表失败:', error)
  }
}

const fetchRecyclePolicy = async () => {
  try {
    const response = await axios.get('/api/storage/recycle-policy')
    if (response.data.success && response.data.policy) {
      recyclePolicy.value = response.data.policy
    }
  } catch (error) {
    console.error('获取循环录制策略失败:', error)
  }
}

const showAddDiskDialog = () => {
  isEdit.value = false
  currentDisk.value = {
    id: '',
    name: '',
    mountPoint: '',
    totalSize: 0,
    usedSize: 0,
    freeSize: 0,
    status: 'online',
    priority: 0,
    enabled: true,
    devicePath: '',
    fileSystem: 'ext4',
    description: ''
  }
  diskDialogVisible.value = true
}

const editDisk = (disk: Disk) => {
  isEdit.value = true
  currentDisk.value = { ...disk }
  diskDialogVisible.value = true
}

const saveDisk = async () => {
  try {
    if (isEdit.value) {
      await axios.put(
        `/api/storage/disks/${currentDisk.value.id}`,
        currentDisk.value
      )
      ElMessage.success('磁盘更新成功')
    } else {
      await axios.post('/api/storage/disks', currentDisk.value)
      ElMessage.success('磁盘添加成功')
    }
    diskDialogVisible.value = false
    fetchDisks()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '操作失败')
  }
}

const updateDisk = async (disk: Disk) => {
  try {
    await axios.put(`/api/storage/disks/${disk.id}`, disk)
    ElMessage.success('磁盘状态更新成功')
    fetchDisks()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '更新失败')
  }
}

const removeDisk = async (disk: Disk) => {
  try {
    await ElMessageBox.confirm(
      `确定要移除磁盘 "${disk.name}" 吗？此操作不会删除磁盘上的数据。`,
      '确认移除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await axios.delete(`/api/storage/disks/${disk.id}`)
    ElMessage.success('磁盘移除成功')
    fetchDisks()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '移除失败')
    }
  }
}

const saveRecyclePolicy = async () => {
  try {
    await axios.put('/api/storage/recycle-policy', recyclePolicy.value)
    ElMessage.success('循环录制策略保存成功')
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

const formatSize = (bytes: number): string => {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${units[i]}`
}

const getProgressColor = (percent: number): string => {
  if (percent < 60) return '#67c23a'
  if (percent < 80) return '#e6a23c'
  return '#f56c6c'
}

const getStatusType = (status: string): string => {
  const types: Record<string, string> = {
    online: 'success',
    offline: 'info',
    full: 'warning',
    error: 'danger'
  }
  return types[status] || 'info'
}

const getStatusText = (status: string): string => {
  const texts: Record<string, string> = {
    online: '在线',
    offline: '离线',
    full: '已满',
    error: '错误'
  }
  return texts[status] || status
}

onMounted(() => {
  fetchDisks()
  fetchRecyclePolicy()
  
  // 定期刷新磁盘状态
  setInterval(fetchDisks, 30000) // 每30秒刷新一次
})
</script>

<style scoped>
.storage-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stats-row {
  margin-bottom: 20px;
}
</style>
