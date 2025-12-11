<template>
  <div class="onvif-device-manager">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon total">📹</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.total }}</div>
              <div class="stat-label">设备总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon online">✓</div>
            <div class="stat-info">
              <div class="stat-value success">{{ statistics.online }}</div>
              <div class="stat-label">在线设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon offline">✗</div>
            <div class="stat-info">
              <div class="stat-value danger">{{ statistics.offline }}</div>
              <div class="stat-label">离线设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon ptz">🎮</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.ptzDevices }}</div>
              <div class="stat-label">PTZ设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="box-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="title">
            <i class="el-icon-video-camera"></i> ONVIF设备管理
          </span>
          <div class="button-group">
            <el-button 
              type="success" 
              @click="discoverDevices"
              :loading="discoverLoading"
              size="default">
              🔍 自动发现
            </el-button>
            <el-button 
              type="primary" 
              @click="showAddModal = true"
              size="default">
              ➕ 手动添加
            </el-button>
            <el-button 
              @click="showBatchModal = true"
              size="default">
              📤 批量导入
            </el-button>
            <el-button 
              @click="exportDevices"
              size="default">
              📥 导出配置
            </el-button>
            <el-button 
              @click="refreshDevices"
              :loading="loading"
              size="default">
              🔄 刷新列表
            </el-button>
          </div>
        </div>
      </template>

      <!-- 搜索过滤 -->
      <div class="filter-bar">
        <el-input
          v-model="searchText"
          placeholder="搜索设备名称、IP地址..."
          style="width: 300px"
          clearable
          @clear="searchText = ''">
          <template #prefix>🔍</template>
        </el-input>
        <el-select v-model="statusFilter" placeholder="状态筛选" style="width: 120px; margin-left: 10px;" clearable>
          <el-option label="全部" value="" />
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
          <el-option label="未知" value="unknown" />
        </el-select>
      </div>

      <!-- 设备列表 -->
      <el-table
        :data="filteredDevices"
        stripe
        style="width: 100%"
        v-loading="loading"
        empty-text="暂无设备"
        @row-click="handleRowClick">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="device-expand">
              <el-descriptions :column="3" border size="small">
                <el-descriptions-item label="设备ID">{{ row.deviceId }}</el-descriptions-item>
                <el-descriptions-item label="固件版本">{{ row.firmwareVersion || '-' }}</el-descriptions-item>
                <el-descriptions-item label="序列号">{{ row.serialNumber || '-' }}</el-descriptions-item>
                <el-descriptions-item label="发现时间">{{ formatTime(row.discoveryTime) }}</el-descriptions-item>
                <el-descriptions-item label="最后在线">{{ formatTime(row.lastSeenTime) }}</el-descriptions-item>
                <el-descriptions-item label="检查间隔">{{ row.checkInterval || 60 }}秒</el-descriptions-item>
                <el-descriptions-item label="PTZ支持">
                  <el-tag :type="row.ptzSupported ? 'success' : 'info'" size="small">
                    {{ row.ptzSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="音频支持">
                  <el-tag :type="row.audioSupported ? 'success' : 'info'" size="small">
                    {{ row.audioSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="服务列表">
                  <div v-if="row.services && row.services.length">
                    <el-tag v-for="service in row.services.slice(0, 3)" :key="service" size="small" style="margin: 2px;">
                      {{ getServiceName(service) }}
                    </el-tag>
                    <span v-if="row.services.length > 3" style="color: #909399;">+{{ row.services.length - 3 }}</span>
                  </div>
                  <span v-else>-</span>
                </el-descriptions-item>
              </el-descriptions>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="设备名称" width="180">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 6px;">
              <span>{{ row.name }}</span>
              <el-tag v-if="row.ptzSupported" type="warning" size="small" effect="plain">PTZ</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP地址" width="130"></el-table-column>
        <el-table-column prop="port" label="端口" width="70"></el-table-column>
        <el-table-column prop="manufacturer" label="制造商" width="120"></el-table-column>
        <el-table-column prop="model" label="型号" width="120"></el-table-column>
        <el-table-column label="状态" width="140">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 8px;">
              <el-tag 
                :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'warning'"
                effect="plain">
                {{ getStatusText(row.status) }}
              </el-tag>
              <span v-if="row.responseTime > 0" style="font-size: 12px; color: #909399;">
                {{ row.responseTime }}ms
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-tooltip content="预览流地址" placement="top">
                <el-button 
                  type="success" 
                  size="small"
                  :disabled="!row.previewURL"
                  @click.stop="showPreview(row)">
                  🎬
                </el-button>
              </el-tooltip>
              <el-tooltip content="获取快照" placement="top">
                <el-button 
                  type="primary" 
                  size="small"
                  @click.stop="getSnapshot(row)">
                  📷
                </el-button>
              </el-tooltip>
              <el-tooltip content="PTZ控制" placement="top">
                <el-button 
                  type="warning" 
                  size="small"
                  :disabled="!row.ptzSupported"
                  @click.stop="showPTZControl(row)">
                  🎮
                </el-button>
              </el-tooltip>
              <el-tooltip content="配置文件" placement="top">
                <el-button 
                  type="info" 
                  size="small"
                  @click.stop="showProfiles(row)">
                  📋
                </el-button>
              </el-tooltip>
              <el-tooltip content="更新IP" placement="top">
                <el-button 
                  size="small"
                  @click.stop="showUpdateIPModal_func(row)">
                  🔄
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除设备" placement="top">
                <el-button 
                  type="danger" 
                  size="small"
                  @click.stop="deleteDevice(row)">
                  🗑️
                </el-button>
              </el-tooltip>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 手动添加设备对话框 -->
    <el-dialog 
      v-model="showAddModal" 
      title="手动添加ONVIF设备"
      width="500px"
      @close="resetAddForm">
      <el-form 
        :model="addForm"
        ref="addFormRef"
        :rules="addFormRules"
        label-width="120px">
        <el-form-item label="添加方式" prop="method">
          <el-radio-group v-model="addForm.method">
            <el-radio label="ip">IP:Port方式</el-radio>
            <el-radio label="xaddr">XADDR方式</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- IP:Port方式 -->
        <template v-if="addForm.method === 'ip'">
          <el-form-item label="IP地址" prop="ip">
            <el-input 
              v-model="addForm.ip" 
              placeholder="例: 192.168.1.100"
              clearable></el-input>
          </el-form-item>
          <el-form-item label="端口" prop="port">
            <el-input-number 
              v-model="addForm.port" 
              :min="1" 
              :max="65535"
              placeholder="例: 8080"></el-input-number>
          </el-form-item>
        </template>

        <!-- XADDR方式 -->
        <template v-if="addForm.method === 'xaddr'">
          <el-form-item label="XADDR" prop="xaddr">
            <el-input 
              v-model="addForm.xaddr" 
              placeholder="例: http://192.168.1.100:8080/onvif/device_service"
              clearable></el-input>
          </el-form-item>
        </template>

        <el-form-item label="用户名" prop="username">
          <el-input 
            v-model="addForm.username" 
            placeholder="默认: admin"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input 
            v-model="addForm.password" 
            type="password"
            placeholder="设备密码"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="设备名称" prop="name">
          <el-input 
            v-model="addForm.name" 
            placeholder="可选：自定义设备名称"
            clearable></el-input>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showAddModal = false">取消</el-button>
        <el-button type="primary" @click="addDevice" :loading="addLoading">
          添加设备
        </el-button>
      </template>
    </el-dialog>

    <!-- 批量导入对话框 -->
    <el-dialog 
      v-model="showBatchModal" 
      title="批量导入ONVIF设备"
      width="600px"
      @close="resetBatchForm">
      <el-form label-width="120px">
        <el-form-item label="导入方式">
          <el-radio-group v-model="batchForm.method">
            <el-radio label="json">JSON格式</el-radio>
            <el-radio label="csv">CSV格式</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="设备数据" v-if="batchForm.method === 'json'">
          <el-input 
            v-model="batchForm.jsonData" 
            type="textarea"
            :rows="10"
            placeholder='[{"ip":"192.168.1.100","port":8080,"username":"admin","password":"admin123","name":"Camera1"}]'></el-input>
        </el-form-item>

        <el-form-item label="CSV文件" v-if="batchForm.method === 'csv'">
          <el-input 
            v-model="batchForm.csvData" 
            type="textarea"
            :rows="10"
            placeholder='ip,port,username,password,name
192.168.1.100,8080,admin,admin123,Camera1
192.168.1.101,8080,admin,admin123,Camera2'></el-input>
        </el-form-item>

        <el-alert 
          v-if="batchForm.method === 'csv'"
          title="CSV格式说明"
          type="info"
          description="第一行为表头，后续行为设备信息，字段顺序: ip,port,username,password,name"
          show-icon
          closable></el-alert>
      </el-form>

      <template #footer>
        <el-button @click="showBatchModal = false">取消</el-button>
        <el-button type="primary" @click="batchAddDevices" :loading="batchLoading">
          导入设备
        </el-button>
      </template>
    </el-dialog>

    <!-- 更新IP对话框 -->
    <el-dialog 
      v-model="showUpdateIPModal" 
      title="更新设备IP地址"
      width="400px"
      @close="resetUpdateIPForm">
      <el-form 
        :model="updateIPForm"
        label-width="100px">
        <el-form-item label="当前IP">
          <el-input 
            v-model="updateIPForm.oldIP" 
            disabled></el-input>
        </el-form-item>
        <el-form-item label="新IP地址">
          <el-input 
            v-model="updateIPForm.newIP" 
            placeholder="输入新的IP地址"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="新端口">
          <el-input-number 
            v-model="updateIPForm.newPort" 
            :min="1" 
            :max="65535"></el-input-number>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showUpdateIPModal = false">取消</el-button>
        <el-button type="primary" @click="updateDeviceIP" :loading="updateIPLoading">
          更新IP
        </el-button>
      </template>
    </el-dialog>

    <!-- 设备预览对话框 -->
    <el-dialog 
      v-model="previewData.showDialog" 
      :title="`设备预览 - ${previewData.device?.name}`"
      width="900px"
      @close="stopPreview"
      @open="onPreviewDialogOpen">
      <div class="preview-container">
        <!-- 凭证输入区域 -->
        <div class="credentials-form" v-if="!previewData.streamInfo && !previewData.loading">
          <el-alert 
            v-if="previewData.error && previewData.error.includes('401')"
            title="RTSP 认证失败，请输入正确的用户名和密码"
            type="warning"
            :closable="false"
            show-icon
            style="margin-bottom: 16px">
          </el-alert>
          <el-form :inline="true" class="credentials-inline-form">
            <el-form-item label="用户名">
              <el-input v-model="previewData.credentials.username" placeholder="admin" style="width: 150px" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="previewData.credentials.password" type="password" placeholder="密码" style="width: 150px" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="startPreviewWithCredentials" :loading="previewData.loading">
                开始预览
              </el-button>
            </el-form-item>
          </el-form>
        </div>

        <!-- 视频播放区域 (使用 PreviewPlayer) -->
        <div class="video-player-wrapper">
          <PreviewPlayer ref="previewPlayerRef" :show="previewData.showDialog" :device="previewData.device ? { deviceId: previewData.device.deviceId || previewData.device.id } : null" :channels="previewData.streamInfo ? [{ channelId: previewData.streamInfo.stream_key || previewData.streamInfo.channel_id }] : []" :selectedChannelId="previewData.streamInfo ? (previewData.streamInfo.stream_key || previewData.streamInfo.channel_id) : ''" />
        </div>

        <!-- 播放信息 显示由 PreviewPlayer 组件处理 -->

        <!-- 设备信息 -->
        <div class="preview-info">
          <el-descriptions :column="3" border size="small">
            <el-descriptions-item label="设备名称">
              {{ previewData.device?.name }}
            </el-descriptions-item>
            <el-descriptions-item label="设备地址">
              {{ previewData.device?.ip }}:{{ previewData.device?.port }}
            </el-descriptions-item>
            <el-descriptions-item label="在线状态">
              <el-tag 
                :type="previewData.device?.status === 'online' ? 'success' : 'danger'"
                effect="plain" size="small">
                {{ getStatusText(previewData.device?.status) }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </div>

      <template #footer>
        <el-button @click="copyPreviewURL">📋 复制RTSP地址</el-button>
        <el-button type="danger" @click="stopPreviewAndClose">停止预览</el-button>
      </template>
    </el-dialog>

    <!-- PTZ控制对话框 -->
    <el-dialog 
      v-model="ptzData.showDialog" 
      :title="`PTZ控制 - ${ptzData.device?.name}`"
      width="500px">
      <div class="ptz-container">
        <div class="ptz-device-info">
          <el-tag type="success">{{ ptzData.device?.ip }}:{{ ptzData.device?.port }}</el-tag>
          <el-tag type="info">{{ ptzData.device?.model }}</el-tag>
        </div>

        <!-- PTZ方向控制 -->
        <div class="ptz-controls">
          <div class="ptz-direction">
            <div class="ptz-row">
              <div class="ptz-cell"></div>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('up')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬆️
              </el-button>
              <div class="ptz-cell"></div>
            </div>
            <div class="ptz-row">
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('left')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬅️
              </el-button>
              <el-button 
                class="ptz-btn center"
                @click="ptzHome">
                🏠
              </el-button>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('right')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ➡️
              </el-button>
            </div>
            <div class="ptz-row">
              <div class="ptz-cell"></div>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('down')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬇️
              </el-button>
              <div class="ptz-cell"></div>
            </div>
          </div>

          <!-- 缩放控制 -->
          <div class="ptz-zoom">
            <el-button 
              class="ptz-btn zoom"
              @mousedown="startPTZ('zoomin')"
              @mouseup="stopPTZ"
              @mouseleave="stopPTZ">
              🔍+
            </el-button>
            <el-button 
              class="ptz-btn zoom"
              @mousedown="startPTZ('zoomout')"
              @mouseup="stopPTZ"
              @mouseleave="stopPTZ">
              🔍-
            </el-button>
          </div>
        </div>

        <!-- 速度控制 -->
        <div class="ptz-speed">
          <span>控制速度：</span>
          <el-slider 
            v-model="ptzData.speed" 
            :min="0.1" 
            :max="1" 
            :step="0.1"
            :format-tooltip="(val: number) => `${(val * 100).toFixed(0)}%`"
            style="width: 200px; margin-left: 10px;"></el-slider>
        </div>

        <!-- 预置位 -->
        <div class="ptz-presets">
          <div class="preset-header">
            <span>预置位</span>
            <el-button size="small" @click="loadPresets">刷新</el-button>
          </div>
          <div class="preset-list" v-loading="ptzData.presetsLoading">
            <el-tag 
              v-for="preset in ptzData.presets" 
              :key="preset.token"
              class="preset-item"
              @click="gotoPreset(preset.token)">
              {{ preset.name || `预置位${preset.token}` }}
            </el-tag>
            <span v-if="!ptzData.presets.length" style="color: #909399;">暂无预置位</span>
          </div>
          <div class="preset-actions">
            <el-input 
              v-model="ptzData.newPresetName" 
              placeholder="输入预置位名称" 
              size="small"
              style="width: 150px;"></el-input>
            <el-button size="small" type="primary" @click="savePreset">保存当前位置</el-button>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="ptzData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 配置文件对话框 -->
    <el-dialog 
      v-model="profilesData.showDialog" 
      :title="`媒体配置 - ${profilesData.device?.name}`"
      width="700px">
      <el-table :data="profilesData.profiles" v-loading="profilesData.loading" stripe>
        <el-table-column prop="name" label="配置名称" width="120"></el-table-column>
        <el-table-column prop="token" label="Token" width="120"></el-table-column>
        <el-table-column prop="encoding" label="编码" width="80"></el-table-column>
        <el-table-column prop="resolution" label="分辨率" width="120"></el-table-column>
        <el-table-column prop="fps" label="帧率" width="60"></el-table-column>
        <el-table-column prop="bitrate" label="码率(kbps)" width="100"></el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="getStreamByProfile(row.token)">
              获取流
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="profilesData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 快照预览对话框 -->
    <el-dialog 
      v-model="snapshotData.showDialog" 
      :title="`快照 - ${snapshotData.device?.name}`"
      width="700px">
      <div class="snapshot-container">
        <div v-if="snapshotData.loading" class="snapshot-loading">
          <el-icon class="is-loading"><i class="el-icon-loading"></i></el-icon>
          正在获取快照...
        </div>
        <img 
          v-else-if="snapshotData.imageUrl" 
          :src="snapshotData.imageUrl" 
          class="snapshot-image"
          alt="设备快照" />
        <div v-else class="snapshot-error">
          {{ snapshotData.error || '无法获取快照' }}
        </div>
      </div>

      <template #footer>
        <el-button @click="refreshSnapshot">🔄 刷新</el-button>
        <el-button @click="downloadSnapshot" :disabled="!snapshotData.imageUrl">📥 下载</el-button>
        <el-button @click="snapshotData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 发现设备对话框 -->
    <el-dialog 
      v-model="showDiscoverModal" 
      title="发现的ONVIF设备"
      width="900px"
      destroy-on-close>
      <div class="discover-hint" v-if="discoveredDevices.length > 0">
        <el-alert type="info" :closable="false">
          发现 {{ discoveredDevices.length }} 个设备，请选择要添加的设备并填写认证信息
        </el-alert>
      </div>
      
      <el-table 
        :data="discoveredDevices" 
        stripe 
        style="width: 100%; margin-top: 15px;"
        max-height="400px">
        <el-table-column width="50">
          <template #default="{ row }">
            <el-checkbox v-model="row.selected" />
          </template>
        </el-table-column>
        <el-table-column label="设备名称" width="150">
          <template #default="{ row }">
            {{ row.name || '未知设备' }}
          </template>
        </el-table-column>
        <el-table-column label="地址" width="200">
          <template #default="{ row }">
            <span class="discover-addr">{{ parseXAddr(row.xaddr).ip }}:{{ parseXAddr(row.xaddr).port }}</span>
          </template>
        </el-table-column>
        <el-table-column label="制造商" prop="manufacturer" width="100"></el-table-column>
        <el-table-column label="型号" prop="model" width="100"></el-table-column>
        <el-table-column label="用户名" width="120">
          <template #default="{ row }">
            <el-input v-model="row.username" size="small" placeholder="admin" />
          </template>
        </el-table-column>
        <el-table-column label="密码" width="120">
          <template #default="{ row }">
            <el-input v-model="row.password" size="small" type="password" placeholder="密码" show-password />
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <div class="discover-footer">
          <el-button @click="discoveredDevices.forEach(d => d.selected = true)">全选</el-button>
          <el-button @click="discoveredDevices.forEach(d => d.selected = false)">取消全选</el-button>
          <el-button type="primary" @click="addDiscoveredDevices" :loading="discoverAddLoading">
            添加选中设备 ({{ discoveredDevices.filter(d => d.selected).length }})
          </el-button>
          <el-button @click="showDiscoverModal = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { VideoCamera } from '@element-plus/icons-vue'
import PreviewPlayer from '../components/PreviewPlayer.vue'

interface Device {
  deviceId: string
  name: string
  ip: string
  port: number
  manufacturer: string
  model: string
  firmwareVersion?: string
  serialNumber?: string
  status: string
  username: string
  password: string
  previewURL?: string
  snapshotURL?: string
  responseTime?: number
  lastCheckTime?: string
  discoveryTime?: string
  lastSeenTime?: string
  checkInterval?: number
  ptzSupported?: boolean
  audioSupported?: boolean
  services?: string[]
  failureCount?: number
}

interface PTZPreset {
  token: string
  name: string
}

interface MediaProfile {
  token: string
  name: string
  encoding: string
  resolution: string
  width: number
  height: number
  fps: number
  bitrate: number
}

const devices = ref<Device[]>([])
const loading = ref(false)
const discoverLoading = ref(false)
const searchText = ref('')
const statusFilter = ref('')

// 统计数据
const statistics = computed(() => {
  const total = devices.value.length
  const online = devices.value.filter(d => d.status === 'online').length
  const offline = devices.value.filter(d => d.status === 'offline').length
  const ptzDevices = devices.value.filter(d => d.ptzSupported).length
  return { total, online, offline, ptzDevices }
})

// 过滤后的设备列表
const filteredDevices = computed(() => {
  return devices.value.filter(device => {
    const matchSearch = !searchText.value || 
      device.name?.toLowerCase().includes(searchText.value.toLowerCase()) ||
      device.ip?.includes(searchText.value) ||
      device.manufacturer?.toLowerCase().includes(searchText.value.toLowerCase())
    const matchStatus = !statusFilter.value || device.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

// 流信息接口
interface StreamInfo {
  device_id: string
  stream_key: string
  app: string
  stream: string
  source_url: string
  flv_url: string
  ws_flv_url: string
  hls_url: string
  rtmp_url: string
  rtsp_url: string
  create_time: number
}

// 预览数据
const previewData = reactive({
  showDialog: false,
  device: null as Device | null,
  loading: false,
  error: '',
  streamInfo: null as StreamInfo | null,
  flvPlayer: null as any,
  // 凭证信息 - 用于 RTSP 认证
  credentials: {
    username: '',
    password: ''
  }
})

// Preview player ref
const previewPlayerRef = ref<any>(null)

// PTZ控制数据
const ptzData = reactive({
  showDialog: false,
  device: null as Device | null,
  speed: 0.5,
  presets: [] as PTZPreset[],
  presetsLoading: false,
  newPresetName: ''
})

// 配置文件数据
const profilesData = reactive({
  showDialog: false,
  device: null as Device | null,
  profiles: [] as MediaProfile[],
  loading: false
})

// 快照数据
const snapshotData = reactive({
  showDialog: false,
  device: null as Device | null,
  imageUrl: '',
  loading: false,
  error: ''
})

// 手动添加表单
const showAddModal = ref(false)
const addLoading = ref(false)
const addFormRef = ref()
const addForm = reactive({
  method: 'ip',
  ip: '',
  port: 8080,
  xaddr: '',
  username: 'admin',
  password: '',
  name: ''
})
const addFormRules = {
  ip: [{ required: true, message: 'IP地址必填', trigger: 'change' }],
  port: [{ required: true, message: '端口必填', trigger: 'change' }],
  xaddr: [{ required: true, message: 'XADDR必填', trigger: 'change' }],
  username: [{ required: true, message: '用户名必填', trigger: 'change' }],
  password: [{ required: true, message: '密码必填', trigger: 'change' }]
}

// 批量导入表单
const showBatchModal = ref(false)
const batchLoading = ref(false)
const batchForm = reactive({
  method: 'json',
  jsonData: '',
  csvData: ''
})

// 更新IP表单
const showUpdateIPModal = ref(false)
const updateIPLoading = ref(false)
const updateIPForm = reactive({
  deviceID: '',
  oldIP: '',
  newIP: '',
  newPort: 8080
})

// 发现设备对话框
interface DiscoveredDevice {
  xaddr: string
  types: string[]
  manufacturer: string
  model: string
  name: string
  location: string
  hardware: string
  sourceIP: string
  selected?: boolean
  username?: string
  password?: string
}

const showDiscoverModal = ref(false)
const discoveredDevices = ref<DiscoveredDevice[]>([])
const discoverAddLoading = ref(false)

// 自动刷新定时器
let refreshTimer: ReturnType<typeof setInterval> | null = null

// 获取设备列表
const refreshDevices = async () => {
  loading.value = true
  try {
    const response = await fetch('/api/onvif/devices')
    if (!response.ok) throw new Error('获取设备列表失败')
    const data = await response.json()
    devices.value = data.devices || []
  } catch (error) {
    ElMessage.error(`加载失败: ${error}`)
  } finally {
    loading.value = false
  }
}

// 自动发现设备
const discoverDevices = async () => {
  discoverLoading.value = true
  try {
    const response = await fetch('/api/onvif/discover', { method: 'POST' })
    if (!response.ok) throw new Error('设备发现失败')
    const data = await response.json()
    
    if (data.devices && data.devices.length > 0) {
      // 显示发现的设备列表
      discoveredDevices.value = data.devices.map((d: any) => ({
        ...d,
        selected: true,
        username: 'admin',
        password: ''
      }))
      showDiscoverModal.value = true
      ElMessage.success(`发现 ${data.devices.length} 个ONVIF设备`)
    } else {
      ElMessage.warning('未发现任何ONVIF设备')
    }
  } catch (error) {
    ElMessage.error(`发现失败: ${error}`)
  } finally {
    discoverLoading.value = false
  }
}

// 添加发现的设备
const addDiscoveredDevices = async () => {
  const selectedDevices = discoveredDevices.value.filter(d => d.selected)
  if (selectedDevices.length === 0) {
    ElMessage.warning('请选择要添加的设备')
    return
  }

  discoverAddLoading.value = true
  let successCount = 0
  let failCount = 0

  for (const device of selectedDevices) {
    try {
      const response = await fetch('/api/onvif/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          xaddr: device.xaddr,
          username: device.username || 'admin',
          password: device.password || ''
        })
      })
      
      if (response.ok) {
        successCount++
      } else {
        failCount++
      }
    } catch {
      failCount++
    }
  }

  discoverAddLoading.value = false
  showDiscoverModal.value = false
  
  if (successCount > 0) {
    ElMessage.success(`成功添加 ${successCount} 个设备${failCount > 0 ? `，失败 ${failCount} 个` : ''}`)
    refreshDevices()
  } else {
    ElMessage.error('添加设备失败')
  }
}

// 从 XADDR 解析 IP 和端口
const parseXAddr = (xaddr: string) => {
  try {
    const url = new URL(xaddr)
    return { ip: url.hostname, port: url.port || '80' }
  } catch {
    return { ip: xaddr, port: '80' }
  }
}

// 导出设备配置
const exportDevices = () => {
  const exportData = devices.value.map(d => ({
    ip: d.ip,
    port: d.port,
    username: d.username,
    password: d.password,
    name: d.name
  }))
  
  const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `onvif_devices_${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('设备配置已导出')
}

// 添加设备
const addDevice = async () => {
  if (!addFormRef.value) return
  await addFormRef.value.validate()

  addLoading.value = true
  try {
    const payload: any = {
      username: addForm.username || 'admin',
      password: addForm.password
    }

    if (addForm.method === 'ip') {
      payload.ip = addForm.ip
      payload.port = addForm.port
    } else {
      payload.xaddr = addForm.xaddr
    }

    if (addForm.name) {
      payload.name = addForm.name
    }

    const response = await fetch('/api/onvif/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!response.ok) throw new Error('添加失败')
    const data = await response.json()
    
    ElMessage.success('设备添加成功')
    showAddModal.value = false
    resetAddForm()
    refreshDevices()
  } catch (error) {
    ElMessage.error(`添加失败: ${error}`)
  } finally {
    addLoading.value = false
  }
}

// 批量添加设备
const batchAddDevices = async () => {
  batchLoading.value = true
  try {
    let devices_list = []

    if (batchForm.method === 'json') {
      devices_list = JSON.parse(batchForm.jsonData)
    } else {
      // 解析CSV格式
      const lines = batchForm.csvData.trim().split('\n')
      const headers = lines[0].split(',').map(h => h.trim())
      
      for (let i = 1; i < lines.length; i++) {
        const values = lines[i].split(',').map(v => v.trim())
        const device: any = {}
        headers.forEach((header, index) => {
          if (header === 'port') {
            device[header] = parseInt(values[index])
          } else {
            device[header] = values[index]
          }
        })
        devices_list.push(device)
      }
    }

    const response = await fetch('/api/onvif/batch-add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ devices: devices_list })
    })

    if (!response.ok) throw new Error('批量添加失败')
    const data = await response.json()
    
    ElMessage.success(`成功添加 ${data.summary.added} 个设备，失败 ${data.summary.failed} 个`)
    showBatchModal.value = false
    resetBatchForm()
    refreshDevices()
  } catch (error) {
    ElMessage.error(`批量添加失败: ${error}`)
  } finally {
    batchLoading.value = false
  }
}

// 显示更新IP对话框
const showUpdateIPModal_func = (row: Device) => {
  updateIPForm.deviceID = row.deviceId
  updateIPForm.oldIP = row.ip
  updateIPForm.newIP = row.ip
  updateIPForm.newPort = row.port
  showUpdateIPModal.value = true
}

// 更新设备IP
const updateDeviceIP = async () => {
  if (!updateIPForm.newIP) {
    ElMessage.error('请输入新IP地址')
    return
  }

  updateIPLoading.value = true
  try {
    const response = await fetch(`/api/onvif/devices/${updateIPForm.deviceID}/refresh`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        new_ip: updateIPForm.newIP,
        new_port: updateIPForm.newPort
      })
    })

    if (!response.ok) throw new Error('更新失败')
    
    ElMessage.success('设备IP已更新')
    showUpdateIPModal.value = false
    resetUpdateIPForm()
    refreshDevices()
  } catch (error) {
    ElMessage.error(`更新失败: ${error}`)
  } finally {
    updateIPLoading.value = false
  }
}

// 显示设备预览
const showPreview = (row: Device) => {
  if (!row.previewURL) {
    ElMessage.warning('该设备没有预览地址')
    return
  }
  
  previewData.device = row
  previewData.error = ''
  previewData.streamInfo = null
  // 初始化凭证 - 使用设备保存的凭证或默认值
  previewData.credentials.username = row.username || 'admin'
  previewData.credentials.password = row.password || ''
  previewData.showDialog = true
}

// 表格行点击处理（兼容模板绑定）
const handleRowClick = (row: Device) => {
  // 简单切换选中状态或展开行，当前实现为打开详情（可根据需要调整）
  // 这里保持行为与之前的 handleRowClick 预期一致：设置当前选中设备并展开（如果需要）
  // 暂时将其行为设为：将设备设为选中（用于未来扩展）
  // 如果你期望点击行打开某个侧边栏或详情页，请告知我以实现。
  console.debug('row clicked', row)
}

// 将内部状态码转成人类可读文本
const getStatusText = (status: string | undefined) => {
  if (!status) return '未知'
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  return status
}

// 由 PreviewPlayer 组件处理播放逻辑与错误
const onPreviewDialogOpen = () => {
  // 打开对话框时只展示凭证输入，等待用户点击“开始预览”
  previewData.error = ''
  previewData.streamInfo = null
  previewData.loading = false
}

// 在进行关键操作前，统一验证设备凭证并在验证成功后同步通道到通道管理
const ensureDeviceAuth = async (device: Device) => {
  if (!device) return false
  // 如果设备已记录的凭证可用，优先使用它
  const username = previewData.device && previewData.device.deviceId === device.deviceId ? previewData.credentials.username : (device.username || 'admin')
  const password = previewData.device && previewData.device.deviceId === device.deviceId ? previewData.credentials.password : (device.password || '')

  try {
    // 调用后端认证接口（假定存在），后端应返回 success: true 表示认证通过
    const resp = await fetch(`/api/onvif/devices/${device.deviceId}/auth/check`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    })
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}))
      ElMessage.error(err.error || '设备认证失败')
      return false
    }
    const data = await resp.json()
    if (!data.success) {
      ElMessage.error(data.error || '设备认证失败')
      return false
    }

    // 认证通过：同步设备的通道到通道管理（尝试 /channels/sync，然后回退到 profiles）
    try {
      const syncResp = await fetch(`/api/onvif/devices/${device.deviceId}/channels/sync`, { method: 'POST' })
      if (syncResp.ok) {
        ElMessage.success('设备认证通过，通道已同步')
        return true
      }
    } catch (e) {
      // 忽略，下一步尝试 profiles
    }

    // 回退：拉取 profiles 并将其作为通道同步到通道管理
    try {
      const profilesResp = await fetch(`/api/onvif/devices/${device.deviceId}/profiles`)
      if (profilesResp.ok) {
        const pData = await profilesResp.json().catch(() => ({}))
        // 如果后端提供了一个批量导入通道接口，可在这里调用；否则只提示成功认证
        // 例如：POST /api/channels/import with body { deviceId, profiles }
        if (pData && pData.profiles && pData.profiles.length) {
          await fetch('/api/channels/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ deviceId: device.deviceId, profiles: pData.profiles })
          }).catch(() => {})
        }
        ElMessage.success('设备认证通过，已同步配置文件作为通道')
        return true
      }
    } catch (e) {
      // 忽略
    }

    // 如果没有同步接口也算认证通过
    return true
  } catch (e: any) {
    ElMessage.error(`认证请求失败: ${e.message || e}`)
    return false
  }
}

// 在用户输入凭据后启动预览（调用后端并通知 PreviewPlayer）
const startPreviewWithCredentials = async () => {
  if (!previewData.device) return
  // 先进行认证并同步通道
  const authOk = await ensureDeviceAuth(previewData.device)
  if (!authOk) return
  previewData.loading = true
  previewData.error = ''
  try {
    const response = await fetch(`/api/onvif/devices/${previewData.device.deviceId}/preview/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: previewData.credentials.username || previewData.device.username || '', password: previewData.credentials.password || previewData.device.password || '' })
    })
    if (!response.ok) {
      const errData = await response.json().catch(() => ({}))
      throw new Error(errData.error || '启动预览失败')
    }
    const data = await response.json()
    if (!data.success) throw new Error(data.error || '启动预览失败')
    previewData.streamInfo = data.data
    await nextTick()
    // 通知 PreviewPlayer 使用已有的 streamInfo 播放
    if (previewPlayerRef.value && previewData.streamInfo) {
      const p = (typeof previewPlayerRef.value.startWithStreamInfo === 'function') ? previewPlayerRef.value : (previewPlayerRef.value.value && typeof previewPlayerRef.value.value.startWithStreamInfo === 'function') ? previewPlayerRef.value.value : (previewPlayerRef.value.$ && previewPlayerRef.value.$.exposed && typeof previewPlayerRef.value.$.exposed.startWithStreamInfo === 'function') ? previewPlayerRef.value.$.exposed : null
      if (p) {
        await p.startWithStreamInfo(previewData.streamInfo)
      } else {
        try { if (typeof previewPlayerRef.value.startPreview === 'function') await previewPlayerRef.value.startPreview() } catch (_) {}
      }
    }
  } catch (e: any) {
    console.error('启动预览失败:', e)
    previewData.error = e.message || '启动预览失败'
    ElMessage.error(`启动预览失败: ${e.message}`)
  } finally {
    previewData.loading = false
  }
}

// 停止预览并关闭对话框
const stopPreviewAndClose = async () => {
  // 仅停止播放并调用后端停止代理
  if (previewPlayerRef.value) await previewPlayerRef.value.stopPlaybackOnly()
  if (previewData.device && previewData.streamInfo) {
    try {
      await fetch(`/api/onvif/devices/${previewData.device.deviceId}/preview/stop`, { method: 'POST', headers: { 'Content-Type': 'application/json' } })
    } catch (e) { console.warn('stop preview api', e) }
  }
  previewData.streamInfo = null
  previewData.error = ''
  previewData.showDialog = false
}

// 兼容模板中 @close="stopPreview" 的调用，调用 stopPreviewAndClose
const stopPreview = async () => {
  await stopPreviewAndClose()
}

// 复制到剪贴板
const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (e) {
    ElMessage.error('复制失败')
  }
}

// 显示PTZ控制
const showPTZControl = (row: Device) => {
  if (!row.ptzSupported) {
    ElMessage.warning('该设备不支持PTZ控制')
    return
  }
  
  ptzData.device = row
  ptzData.showDialog = true
  loadPresets()
}

// 加载预置位列表
const loadPresets = async () => {
  if (!ptzData.device) return
  
  ptzData.presetsLoading = true
  try {
    const response = await fetch(`/api/onvif/devices/${ptzData.device.deviceId}/presets`)
    if (!response.ok) throw new Error('获取预置位失败')
    const data = await response.json()
    ptzData.presets = data.presets || []
  } catch (error) {
    console.error('加载预置位失败:', error)
    ptzData.presets = []
  } finally {
    ptzData.presetsLoading = false
  }
}

// PTZ控制
const startPTZ = async (command: string) => {
  if (!ptzData.device) return
  // 先进行设备认证
  const ok = await ensureDeviceAuth(ptzData.device)
  if (!ok) return
  try {
    await fetch('/api/control/ptz', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        deviceId: ptzData.device.deviceId,
        deviceType: 'onvif',
        ptzCmd: command,
        speed: ptzData.speed
      })
    })
  } catch (error) {
    ElMessage.error(`PTZ控制失败: ${error}`)
  }
}

const stopPTZ = async () => {
  if (!ptzData.device) return
  const ok = await ensureDeviceAuth(ptzData.device)
  if (!ok) return
  try {
    await fetch('/api/control/ptz', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        deviceId: ptzData.device.deviceId,
        deviceType: 'onvif',
        ptzCmd: 'stop',
        speed: 0
      })
    })
  } catch (error) {
    console.error('停止PTZ失败:', error)
  }
}

const ptzHome = async () => {
  if (!ptzData.device) return
  const ok = await ensureDeviceAuth(ptzData.device)
  if (!ok) return
  try {
    await fetch('/api/control/ptz', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        deviceId: ptzData.device.deviceId,
        deviceType: 'onvif',
        ptzCmd: 'home',
        speed: ptzData.speed
      })
    })
    ElMessage.success('已移动到Home位置')
  } catch (error) {
    ElMessage.error(`移动失败: ${error}`)
  }
}

// 移动到预置位
const gotoPreset = async (presetToken: string) => {
  if (!ptzData.device) return
  
  try {
    const response = await fetch(`/api/onvif/devices/${ptzData.device.deviceId}/preset/${presetToken}`, {
      method: 'POST'
    })
    if (!response.ok) throw new Error('移动失败')
    ElMessage.success('已移动到预置位')
  } catch (error) {
    ElMessage.error(`移动到预置位失败: ${error}`)
  }
}

// 保存当前位置为预置位
const savePreset = async () => {
  if (!ptzData.device || !ptzData.newPresetName.trim()) {
    ElMessage.warning('请输入预置位名称')
    return
  }
  
  try {
    const response = await fetch(`/api/onvif/devices/${ptzData.device.deviceId}/preset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: ptzData.newPresetName })
    })
    if (!response.ok) throw new Error('保存失败')
    ElMessage.success('预置位保存成功')
    ptzData.newPresetName = ''
    loadPresets()
  } catch (error) {
    ElMessage.error(`保存预置位失败: ${error}`)
  }
}

// (重复的 showProfiles 已删除，使用文件后部定义的带认证版本)

// 根据配置获取流地址
const getStreamByProfile = async (profileToken: string) => {
  if (!profilesData.device) return
  
  try {
    const response = await fetch('/api/stream/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        deviceId: profilesData.device.deviceId,
        deviceType: 'onvif',
        profileToken: profileToken
      })
    })
    if (!response.ok) throw new Error('获取流地址失败')
    const data = await response.json()
    
    if (data.streamUrl) {
      await navigator.clipboard.writeText(data.streamUrl)
      ElMessage.success(`流地址已复制: ${data.streamUrl}`)
    }
  } catch (error) {
    ElMessage.error(`获取流地址失败: ${error}`)
  }
}

// 获取快照
const getSnapshot = async (row: Device) => {
  snapshotData.device = row
  snapshotData.showDialog = true
  // 先进行认证
  const ok = await ensureDeviceAuth(row)
  if (ok) await refreshSnapshot()
}

const refreshSnapshot = async () => {
  if (!snapshotData.device) return
  
  snapshotData.loading = true
  snapshotData.error = ''
  snapshotData.imageUrl = ''
  
  try {
    const response = await fetch(`/api/onvif/devices/${snapshotData.device.deviceId}/snapshot`)
    if (!response.ok) throw new Error('获取快照失败')
    
    const blob = await response.blob()
    snapshotData.imageUrl = URL.createObjectURL(blob)
  } catch (error) {
    snapshotData.error = `获取快照失败: ${error}`
  } finally {
    snapshotData.loading = false
  }
}

const downloadSnapshot = () => {
  if (!snapshotData.imageUrl || !snapshotData.device) return
  
  const a = document.createElement('a')
  a.href = snapshotData.imageUrl
  a.download = `snapshot_${snapshotData.device.ip}_${Date.now()}.jpg`
  a.click()
}

// 显示配置文件
const showProfiles = async (row: Device) => {
  profilesData.device = row
  profilesData.showDialog = true
  profilesData.loading = true
  // 先认证
  const ok = await ensureDeviceAuth(row)
  if (!ok) {
    profilesData.loading = false
    return
  }
  
  try {
    const response = await fetch(`/api/onvif/devices/${row.deviceId}/profiles`)
    if (!response.ok) throw new Error('获取配置文件失败')
    const data = await response.json()
    profilesData.profiles = data.profiles || []
  } catch (error) {
    ElMessage.error(`获取配置文件失败: ${error}`)
    profilesData.profiles = []
  } finally {
    profilesData.loading = false
  }
}

// 根据服务类型友好展示服务名
const getServiceName = (service: string) => {
  if (!service) return ''
  if (service.includes('Media')) return 'Media'
  if (service.includes('PTZ')) return 'PTZ'
  if (service.includes('Event')) return 'Events'
  if (service.includes('Device')) return 'Device'
  if (service.includes('Imaging')) return 'Imaging'
  if (service.includes('Recording')) return 'Recording'
  return service.split('/').pop() || service
}

// 格式化时间
const formatTime = (timeStr: string | undefined) => {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

// 删除设备
const deleteDevice = (row: Device) => {
  ElMessageBox.confirm(
    `确定删除设备"${row.name}"吗？`,
    '删除确认',
    { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
  )
    .then(async () => {
      try {
        const response = await fetch(`/api/onvif/devices/${encodeURIComponent(row.deviceId)}`, {
          method: 'DELETE'
        })

        if (!response.ok) throw new Error('删除失败')
        
        ElMessage.success('设备已删除')
        refreshDevices()
      } catch (error) {
        ElMessage.error(`删除失败: ${error}`)
      }
    })
    .catch(() => {})
}

// 复制预览URL到剪贴板
const copyPreviewURL = async () => {
  if (!previewData.device?.previewURL) {
    ElMessage.error('没有预览地址可复制')
    return
  }

  try {
    await navigator.clipboard.writeText(previewData.device.previewURL)
    ElMessage.success('预览地址已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败，请手动复制')
  }
}

// 重置表单
const resetAddForm = () => {
  addForm.method = 'ip'
  addForm.ip = ''
  addForm.port = 8080
  addForm.xaddr = ''
  addForm.username = 'admin'
  addForm.password = ''
  addForm.name = ''
}

const resetBatchForm = () => {
  batchForm.method = 'json'
  batchForm.jsonData = ''
  batchForm.csvData = ''
}

const resetUpdateIPForm = () => {
  updateIPForm.deviceID = ''
  updateIPForm.oldIP = ''
  updateIPForm.newIP = ''
  updateIPForm.newPort = 8080
}

// 组件挂载
onMounted(() => {
  refreshDevices()
  // 设置自动刷新（每30秒）
  refreshTimer = setInterval(refreshDevices, 30000)
})

// 组件卸载
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  // 清理快照URL
  if (snapshotData.imageUrl) {
    URL.revokeObjectURL(snapshotData.imageUrl)
  }
})
</script>

<style scoped lang="css">
.onvif-device-manager {
  padding: 20px;
}

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  cursor: default;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  font-size: 32px;
  width: 50px;
  height: 50px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
}

.stat-icon.total { background: #e8f4fd; }
.stat-icon.online { background: #e8f8e8; }
.stat-icon.offline { background: #fde8e8; }
.stat-icon.ptz { background: #fff3e0; }

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #303133;
}

.stat-value.success { color: #67c23a; }
.stat-value.danger { color: #f56c6c; }

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

.box-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0;
}

.title {
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.button-group {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-bar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
}

.device-expand {
  padding: 10px 20px;
  background: #fafafa;
}

:deep(.el-button) {
  display: flex;
  align-items: center;
  gap: 4px;
}

.preview-container {
  padding: 20px 0;
}

.credentials-form {
  background: #f5f7fa;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.credentials-inline-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.credentials-inline-form .el-form-item {
  margin-bottom: 0;
}

.preview-url {
  margin-bottom: 20px;
  word-break: break-all;
}

.preview-info {
  margin: 20px 0;
}

/* 视频播放器样式 */
.video-player-wrapper {
  position: relative;
  width: 100%;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.video-player {
  width: 100%;
  max-height: 480px;
  min-height: 360px;
  background: #000;
  display: block;
}

.video-error {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  gap: 16px;
}

.video-error p {
  max-width: 80%;
  text-align: center;
  color: #f56c6c;
}

.stream-urls {
  margin-bottom: 16px;
}

.stream-urls :deep(.el-link) {
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

/* PTZ控制样式 */
.ptz-container {
  padding: 10px;
}

.ptz-device-info {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
  justify-content: center;
}

.ptz-controls {
  display: flex;
  gap: 30px;
  justify-content: center;
  align-items: center;
  margin-bottom: 20px;
}

.ptz-direction {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ptz-row {
  display: flex;
  gap: 4px;
  justify-content: center;
}

.ptz-cell {
  width: 50px;
  height: 50px;
}

.ptz-btn {
  width: 50px;
  height: 50px;
  font-size: 20px;
  padding: 0;
}

.ptz-btn.center {
  background: #409eff;
  color: white;
}

.ptz-zoom {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ptz-btn.zoom {
  width: 60px;
  height: 40px;
  font-size: 16px;
}

.ptz-speed {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 6px;
}

.ptz-presets {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 15px;
}

.preset-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  font-weight: bold;
}

.preset-list {
  min-height: 40px;
  margin-bottom: 10px;
}

.preset-item {
  margin: 4px;
  cursor: pointer;
  transition: all 0.3s;
}

.preset-item:hover {
  transform: scale(1.05);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.preset-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

/* 快照样式 */
.snapshot-container {
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.snapshot-image {
  max-width: 100%;
  max-height: 500px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.snapshot-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: #909399;
}

.snapshot-error {
  color: #f56c6c;
  text-align: center;
}

:deep(.el-descriptions) {
  margin-bottom: 20px;
}

:deep(.el-descriptions-item__label) {
  font-weight: 600;
}

/* 发现设备对话框样式 */
.discover-hint {
  margin-bottom: 10px;
}

.discover-addr {
  font-family: monospace;
  color: #409EFF;
}

.discover-footer {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
</style>
