<template>
  <div class="settings-container">
    <div class="page-header">
      <h1>系统配置</h1>
      <div class="header-actions">
        <el-button type="info" size="small" @click="loadSettings">
          <el-icon><Refresh /></el-icon>
          重新加载
        </el-button>
        <el-button type="primary" size="small" @click="saveSettings">
          <el-icon><Check /></el-icon>
          保存配置
        </el-button>
      </div>
    </div>
    
    <el-row :gutter="20" class="settings-grid">
      <!-- GB28181 配置 -->
      <el-col :xs="24" :lg="8">
        <el-card class="settings-section" shadow="hover">
          <template #header>
            <div class="section-header">
              <el-icon><VideoCamera /></el-icon>
              <span>GB28181 配置</span>
              <el-switch
                v-model="serviceStatus.gb28181"
                :loading="serviceLoading.gb28181"
                active-text="运行中"
                inactive-text="已停止"
                style="margin-left: auto;"
                @change="toggleGB28181Service"
              />
            </div>
          </template>
          <el-form :model="config.GB28181" label-width="120px" size="small">
            <el-form-item label="SIP IP">
              <el-input v-model="config.GB28181.SipIP" placeholder="0.0.0.0" />
            </el-form-item>
            
            <el-form-item label="SIP 端口">
              <el-input-number 
                v-model="config.GB28181.SipPort" 
                :min="1" 
                :max="65535" 
                controls-position="right"
              />
            </el-form-item>
            
            <el-form-item label="Realm">
              <el-input v-model="config.GB28181.Realm" />
            </el-form-item>
            
            <el-form-item label="服务器 ID">
              <el-input v-model="config.GB28181.ServerID" />
            </el-form-item>
            
            <el-form-item label="密码">
              <el-input v-model="config.GB28181.Password" type="password" show-password placeholder="SIP 认证密码" />
            </el-form-item>
            
            <el-form-item label="心跳间隔 (秒)">
              <el-input-number 
                v-model="config.GB28181.HeartbeatInterval" 
                :min="10" 
                :max="3600" 
                controls-position="right"
              />
            </el-form-item>
            
            <el-form-item label="注册过期时间 (秒)">
              <el-input-number 
                v-model="config.GB28181.RegisterExpires" 
                :min="60" 
                :max="86400" 
                controls-position="right"
              />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      
      <!-- ONVIF 配置 -->
      <el-col :xs="24" :lg="8">
        <el-card class="settings-section" shadow="hover">
          <template #header>
            <div class="section-header">
              <el-icon><Monitor /></el-icon>
              <span>ONVIF 配置</span>
              <el-switch
                v-model="serviceStatus.onvif"
                :loading="serviceLoading.onvif"
                active-text="运行中"
                inactive-text="已停止"
                style="margin-left: auto;"
                @change="toggleONVIFService"
              />
            </div>
          </template>
          <el-form :model="config.ONVIF" label-width="120px" size="small">
            <el-form-item label="发现间隔 (秒)">
              <el-input-number 
                v-model="config.ONVIF.DiscoveryInterval" 
                :min="10" 
                :max="3600" 
                controls-position="right"
              />
            </el-form-item>
            
            <el-form-item label="媒体流端口范围">
              <el-input v-model="config.ONVIF.MediaPortRange" placeholder="8000-9000" />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      
      <!-- API 配置 -->
      <el-col :xs="24" :lg="8">
        <el-card class="settings-section" shadow="hover">
          <template #header>
            <div class="section-header">
              <el-icon><Setting /></el-icon>
              <span>API 配置</span>
            </div>
          </template>
          <el-form :model="config.API" label-width="120px" size="small">
            <el-form-item label="主机">
              <el-input v-model="config.API.Host" placeholder="0.0.0.0" />
            </el-form-item>
            
            <el-form-item label="端口">
              <el-input-number 
                v-model="config.API.Port" 
                :min="1" 
                :max="65535" 
                controls-position="right"
              />
            </el-form-item>
            
            <el-form-item label="CORS 允许源">
              <el-input v-model="corsOrigins" placeholder="*" />
              <template #label>
                <span>CORS 允许源</span>
                <el-tooltip content="多个源用逗号分隔" placement="top">
                  <el-icon><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
    
    <!-- AI 智能录像配置 -->
    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :span="24">
        <el-card class="settings-section" shadow="hover">
          <template #header>
            <div class="section-header">
              <el-icon><Monitor /></el-icon>
              <span>AI 智能录像</span>
            </div>
          </template>
          <el-form :model="config.AI" label-width="140px" size="small">
            <el-row :gutter="20">
              <el-col :span="8">
                <el-form-item label="启用AI录像">
                  <el-switch v-model="config.AI.Enable" />
                  <div class="form-tip">
                    开启后可根据AI检测结果自动控制录像
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="置信度阈值">
                    <el-slider 
                    v-model="config.AI.Confidence" 
                    :min="0" 
                    :max="1" 
                    :step="0.05"
                    :format-tooltip="(val: number) => (val * 100).toFixed(0) + '%'"
                  />
                  <div class="form-tip">
                    越高越准确但可能漏检，建议0.5-0.7
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="检测间隔(秒)">
                  <el-input-number 
                    v-model="config.AI.DetectInterval" 
                    :min="1" 
                    :max="60"
                    controls-position="right"
                    style="width: 100%"
                  />
                  <div class="form-tip">
                    间隔越短越实时，但占用更多资源
                  </div>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :span="12">
                <el-form-item label="AI服务地址">
                  <el-input 
                    v-model="config.AI.APIEndpoint" 
                    placeholder="http://localhost:8000/detect"
                  />
                  <div class="form-tip">
                    外部AI检测服务API地址
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="录像延迟(秒)">
                  <el-input-number 
                    v-model="config.AI.RecordDelay" 
                    :min="0" 
                    :max="300"
                    controls-position="right"
                    style="width: 100%"
                  />
                  <div class="form-tip">
                    检测到人后继续录制的时长
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="最小录像时长(秒)">
                  <el-input-number 
                    v-model="config.AI.MinRecordTime" 
                    :min="1" 
                    :max="60"
                    controls-position="right"
                    style="width: 100%"
                  />
                  <div class="form-tip">
                    避免产生过多碎片文件
                  </div>
                </el-form-item>
              </el-col>
            </el-row>

            <el-alert 
              title="AI录像说明" 
              type="info" 
              :closable="false"
              style="margin-top: 10px"
            >
              <ul style="margin: 5px 0; padding-left: 20px;">
                <li>启用后，系统将通过AI检测视频流中是否有人</li>
                <li>检测到人时自动开始录像，无人时自动停止</li>
                <li>需要部署外部AI检测服务或使用云服务API</li>
                <li>可大幅节省存储空间（减少60-80%无效录像）</li>
              </ul>
            </el-alert>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
    
    <div v-if="message" class="message-container">
      <el-alert 
        :title="message.text" 
        :type="message.type" 
        :closable="false"
        show-icon
      />
    </div>

    <!-- ZLM 媒体服务器配置 -->
    <el-card class="settings-section zlm-section" shadow="hover">
      <template #header>
        <div class="section-header">
          <el-icon><VideoPlay /></el-icon>
          <span>ZLM 媒体服务器配置</span>
        </div>
      </template>
      
      <el-tabs v-model="zlmActiveTab">
        <!-- 基础配置 -->
        <el-tab-pane label="基础配置" name="basic">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="使用嵌入式">
                <el-switch v-model="config.ZLM.UseEmbedded" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="自动重启">
                <el-switch v-model="config.ZLM.AutoRestart" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="最大重启次数">
                <el-input-number v-model="config.ZLM.MaxRestarts" :min="1" :max="100" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- API 配置 -->
        <el-tab-pane label="API" name="api">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="调试模式">
                <el-switch v-model="config.ZLM.API.Debug" />
              </el-form-item>
            </el-col>
            <el-col :span="16">
              <el-form-item label="API Secret">
                <el-input v-model="config.ZLM.API.Secret" placeholder="API 密钥" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- HTTP 配置 -->
        <el-tab-pane label="HTTP" name="http">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="HTTP 端口">
                <el-input-number v-model="config.ZLM.HTTP.Port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="SSL 端口">
                <el-input-number v-model="config.ZLM.HTTP.SSLPort" :min="0" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="目录菜单">
                <el-switch v-model="config.ZLM.HTTP.DirMenu" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="根目录">
                <el-input v-model="config.ZLM.HTTP.RootPath" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="允许跨域">
                <el-switch v-model="config.ZLM.HTTP.AllowCrossDomains" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- RTSP 配置 -->
        <el-tab-pane label="RTSP" name="rtsp">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="RTSP 端口">
                <el-input-number v-model="config.ZLM.RTSP.Port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="SSL 端口">
                <el-input-number v-model="config.ZLM.RTSP.SSLPort" :min="0" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="直接代理">
                <el-switch v-model="config.ZLM.RTSP.DirectProxy" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="低延迟">
                <el-switch v-model="config.ZLM.RTSP.LowLatency" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="Basic 认证">
                <el-switch v-model="config.ZLM.RTSP.AuthBasic" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- RTMP 配置 -->
        <el-tab-pane label="RTMP" name="rtmp">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="RTMP 端口">
                <el-input-number v-model="config.ZLM.RTMP.Port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="SSL 端口">
                <el-input-number v-model="config.ZLM.RTMP.SSLPort" :min="0" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="直接代理">
                <el-switch v-model="config.ZLM.RTMP.DirectProxy" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- RTP 代理配置 -->
        <el-tab-pane label="RTP代理" name="rtpproxy">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="RTP 端口">
                <el-input-number v-model="config.ZLM.RTPProxy.Port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="超时时间(秒)">
                <el-input-number v-model="config.ZLM.RTPProxy.TimeoutSec" :min="1" :max="300" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="端口范围">
                <el-input v-model="config.ZLM.RTPProxy.PortRange" placeholder="30000-35000" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- 协议配置 -->
        <el-tab-pane label="协议转换" name="protocol">
          <el-row :gutter="20">
            <el-col :span="6">
              <el-form-item label="启用音频">
                <el-switch v-model="config.ZLM.Protocol.EnableAudio" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="添加静音">
                <el-switch v-model="config.ZLM.Protocol.AddMuteAudio" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="启用 HLS">
                <el-switch v-model="config.ZLM.Protocol.EnableHLS" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="启用 MP4">
                <el-switch v-model="config.ZLM.Protocol.EnableMP4" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :span="6">
              <el-form-item label="启用 RTSP">
                <el-switch v-model="config.ZLM.Protocol.EnableRTSP" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="启用 RTMP">
                <el-switch v-model="config.ZLM.Protocol.EnableRTMP" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="启用 TS">
                <el-switch v-model="config.ZLM.Protocol.EnableTS" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="启用 FMP4">
                <el-switch v-model="config.ZLM.Protocol.EnableFMP4" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- 录像配置 -->
        <el-tab-pane label="录像" name="record">
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="录像目录">
                <el-input v-model="config.ZLM.Record.RecordPath" placeholder="./recordings">
                  <template #prepend>路径</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="应用名称">
                <el-input v-model="config.ZLM.Record.AppName" placeholder="record" />
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-divider content-position="left">录像分割配置</el-divider>
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="按时间分割(秒)">
                <el-input-number 
                  v-model="config.ZLM.Record.FileSecond" 
                  :min="0" 
                  :max="86400"
                  :step="300"
                  controls-position="right"
                  style="width: 100%"
                >
                  <template #append>秒</template>
                </el-input-number>
                <div class="form-tip">
                  0=不分割, 3600=每小时, 1800=每半小时
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="按大小分割(MB)">
                <el-input-number 
                  v-model="config.ZLM.Record.FileSizeMB" 
                  :min="0" 
                  :max="10240"
                  :step="100"
                  controls-position="right"
                  style="width: 100%"
                >
                  <template #append>MB</template>
                </el-input-number>
                <div class="form-tip">
                  0=不限制, 建议500-1000MB
                </div>
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-divider content-position="left">高级选项</el-divider>
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="采样间隔(ms)">
                <el-input-number v-model="config.ZLM.Record.SampleMS" :min="100" :max="5000" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="快速启动">
                <el-switch v-model="config.ZLM.Record.FastStart" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="启用FMP4">
                <el-switch v-model="config.ZLM.Record.EnableFmp4" />
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-alert 
            title="录像文件命名规则" 
            type="info" 
            :closable="false"
            style="margin-top: 15px"
          >
            录像文件按以下格式保存：{RecordPath}/{AppName}/{ChannelID}/{Date}/YYYY-MM-DD-HH-MM-SS-{index}.mp4
          </el-alert>
        </el-tab-pane>

        <!-- WebRTC 配置 -->
        <el-tab-pane label="WebRTC" name="rtc">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="UDP 端口">
                <el-input-number v-model="config.ZLM.RTC.Port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="TCP 端口">
                <el-input-number v-model="config.ZLM.RTC.TCPPort" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="超时时间(秒)">
                <el-input-number v-model="config.ZLM.RTC.TimeoutSec" :min="1" :max="300" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="外部 IP">
                <el-input v-model="config.ZLM.RTC.ExternIP" placeholder="留空自动检测" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  Refresh,
  Check,
  VideoCamera,
  Monitor,
  Setting,
  InfoFilled,
  VideoPlay
} from '@element-plus/icons-vue'
import { api } from '../lib/api'

interface GB28181Config {
  SipIP: string
  SipPort: number
  Realm: string
  ServerID: string
  Password: string
  HeartbeatInterval: number
  RegisterExpires: number
}

interface ONVIFConfig {
  DiscoveryInterval: number
  MediaPortRange: string
}

interface APIConfig {
  Host: string
  Port: number
  CorsAllowOrigins: string[]
}

interface ZLMConfig {
  UseEmbedded: boolean
  AutoRestart: boolean
  MaxRestarts: number
  API: {
    Debug: boolean
    Secret: string
    SnapRoot: string
    DefaultSnap: string
  }
  HTTP: {
    Port: number
    SSLPort: number
    RootPath: string
    DirMenu: boolean
    AllowCrossDomains: boolean
  }
  RTSP: {
    Port: number
    SSLPort: number
    DirectProxy: boolean
    LowLatency: boolean
    AuthBasic: boolean
  }
  RTMP: {
    Port: number
    SSLPort: number
    DirectProxy: boolean
  }
  RTPProxy: {
    Port: number
    TimeoutSec: number
    PortRange: string
  }
  Protocol: {
    EnableAudio: boolean
    AddMuteAudio: boolean
    EnableHLS: boolean
    EnableMP4: boolean
    EnableRTSP: boolean
    EnableRTMP: boolean
    EnableTS: boolean
    EnableFMP4: boolean
  }
  Record: {
    RecordPath: string
    AppName: string
    SampleMS: number
    FastStart: boolean
    EnableFmp4: boolean
    FileSecond: number
    FileSizeMB: number
  }
  RTC: {
    Port: number
    TCPPort: number
    TimeoutSec: number
    ExternIP: string
  }
}

interface AIConfig {
  Enable: boolean
  APIEndpoint: string
  ModelPath: string
  Confidence: number
  DetectInterval: number
  RecordDelay: number
  MinRecordTime: number
}

interface SystemConfig {
  GB28181: GB28181Config
  ONVIF: ONVIFConfig
  API: APIConfig
  ZLM: ZLMConfig
  AI: AIConfig
}

interface Message {
  type: 'success' | 'error' | 'info'
  text: string
}

const zlmActiveTab = ref('basic')

const config = ref<SystemConfig>({
  GB28181: {
    SipIP: '0.0.0.0',
    SipPort: 5060,
    Realm: '3402000000',
    ServerID: '34020000002000000001',
    Password: '12345678',
    HeartbeatInterval: 60,
    RegisterExpires: 3600
  },
  ONVIF: {
    DiscoveryInterval: 60,
    MediaPortRange: '8000-9000'
  },
  API: {
    Host: '0.0.0.0',
    Port: 9080,
    CorsAllowOrigins: ['*']
  },
  ZLM: {
    UseEmbedded: true,
    AutoRestart: true,
    MaxRestarts: 10,
    API: {
      Debug: false,
      Secret: '',
      SnapRoot: './www/snap/',
      DefaultSnap: './www/logo.png'
    },
    HTTP: {
      Port: 8080,
      SSLPort: 8443,
      RootPath: './www',
      DirMenu: true,
      AllowCrossDomains: true
    },
    RTSP: {
      Port: 8554,
      SSLPort: 0,
      DirectProxy: true,
      LowLatency: false,
      AuthBasic: false
    },
    RTMP: {
      Port: 1935,
      SSLPort: 0,
      DirectProxy: true
    },
    RTPProxy: {
      Port: 10000,
      TimeoutSec: 15,
      PortRange: '30000-35000'
    },
    Protocol: {
      EnableAudio: true,
      AddMuteAudio: true,
      EnableHLS: true,
      EnableMP4: false,
      EnableRTSP: true,
      EnableRTMP: true,
      EnableTS: true,
      EnableFMP4: true
    },
    Record: {
      RecordPath: './recordings',
      AppName: 'record',
      SampleMS: 500,
      FastStart: false,
      EnableFmp4: false,
      FileSecond: 3600,
      FileSizeMB: 0
    },
    RTC: {
      Port: 8000,
      TCPPort: 8000,
      TimeoutSec: 15,
      ExternIP: ''
    }
  },
  AI: {
    Enable: false,
    APIEndpoint: 'http://localhost:8000/detect',
    ModelPath: './models/yolov8n.onnx',
    Confidence: 0.5,
    DetectInterval: 2,
    RecordDelay: 10,
    MinRecordTime: 5
  }
})

const corsOrigins = computed({
  get: () => config.value.API.CorsAllowOrigins.join(','),
  set: (value) => {
    config.value.API.CorsAllowOrigins = value.split(',').map(origin => origin.trim())
  }
})

// 服务状态管理
const serviceStatus = ref({
  gb28181: true,
  onvif: true
})

const serviceLoading = ref({
  gb28181: false,
  onvif: false
})

const message = ref<Message | null>(null)

const showMessage = (type: Message['type'], text: string) => {
  message.value = { type, text }
  setTimeout(() => {
    message.value = null
  }, 3000)
}

const loadSettings = async () => {
  try {
    const response = await api.get('/api/config')
    if (response.ok) {
      const data = await response.json()
      // 深度合并配置，保留默认值
      if (data.GB28181) {
        config.value.GB28181 = { ...config.value.GB28181, ...data.GB28181 }
      }
      if (data.ONVIF) {
        config.value.ONVIF = { ...config.value.ONVIF, ...data.ONVIF }
      }
      if (data.API) {
        config.value.API = { ...config.value.API, ...data.API }
      }
      if (data.ZLM) {
        config.value.ZLM = {
          ...config.value.ZLM,
          ...data.ZLM,
          API: { ...config.value.ZLM.API, ...(data.ZLM.API || {}) },
          HTTP: { ...config.value.ZLM.HTTP, ...(data.ZLM.HTTP || {}) },
          RTSP: { ...config.value.ZLM.RTSP, ...(data.ZLM.RTSP || {}) },
          RTMP: { ...config.value.ZLM.RTMP, ...(data.ZLM.RTMP || {}) },
          RTPProxy: { ...config.value.ZLM.RTPProxy, ...(data.ZLM.RTPProxy || {}) },
          Protocol: { ...config.value.ZLM.Protocol, ...(data.ZLM.Protocol || {}) },
          Record: { ...config.value.ZLM.Record, ...(data.ZLM.Record || {}) },
          RTC: { ...config.value.ZLM.RTC, ...(data.ZLM.RTC || {}) }
        }
      }
      if (data.AI) {
        config.value.AI = { ...config.value.AI, ...data.AI }
      }
      showMessage('success', '配置加载成功')
    } else {
      throw new Error('Failed to load settings')
    }
  } catch (error) {
    showMessage('error', '加载配置失败')
    console.error('Error loading settings:', error)
  }
}

const saveSettings = async () => {
  try {
    const response = await api.put('/api/config', config.value)
    
    if (response.ok) {
      showMessage('success', '配置保存成功')
    } else {
      throw new Error('Failed to save settings')
    }
  } catch (error) {
    showMessage('error', '保存配置失败')
    console.error('Error saving settings:', error)
  }
}

// 加载服务状态
const loadServiceStatus = async () => {
  try {
    const response = await api.get('/api/services/status')
    if (response.ok) {
      const data = await response.json()
      serviceStatus.value.gb28181 = data.gb28181?.enabled ?? true
      serviceStatus.value.onvif = data.onvif?.enabled ?? true
    }
  } catch (error) {
    console.error('加载服务状态失败:', error)
  }
}

// 切换 GB28181 服务
const toggleGB28181Service = async (enabled: boolean) => {
  serviceLoading.value.gb28181 = true
  try {
    const response = await api.post('/api/services/gb28181/control', { action: enabled ? 'start' : 'stop' })
    
    if (response.ok) {
      showMessage('success', `GB28181 服务已${enabled ? '启动' : '停止'}`)
    } else {
      const data = await response.json()
      throw new Error(data.error || '操作失败')
    }
  } catch (error: any) {
    showMessage('error', `操作失败: ${error.message}`)
    // 回滚状态
    serviceStatus.value.gb28181 = !enabled
  } finally {
    serviceLoading.value.gb28181 = false
  }
}

// 切换 ONVIF 服务
const toggleONVIFService = async (enabled: boolean) => {
  serviceLoading.value.onvif = true
  try {
    const response = await api.post('/api/services/onvif/control', { action: enabled ? 'start' : 'stop' })
    
    if (response.ok) {
      showMessage('success', `ONVIF 服务已${enabled ? '启动' : '停止'}`)
    } else {
      const data = await response.json()
      throw new Error(data.error || '操作失败')
    }
  } catch (error: any) {
    showMessage('error', `操作失败: ${error.message}`)
    // 回滚状态
    serviceStatus.value.onvif = !enabled
  } finally {
    serviceLoading.value.onvif = false
  }
}




onMounted(() => {
  loadSettings()
  loadServiceStatus()
})
</script>

<style scoped>
.settings-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
  padding-bottom: 15px;
  border-bottom: 1px solid #e0e0e0;
}

.page-header h1 {
  margin: 0;
  color: #303133;
  font-size: 24px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.settings-grid {
  margin-bottom: 30px;
}

.settings-section {
  height: 100%;
  transition: transform 0.3s ease, box-shadow 0.3s ease;
}

.settings-section:hover {
  transform: translateY(-2px);
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: #303133;
}

.section-header .el-icon {
  color: #409eff;
  font-size: 18px;
}

:deep(.el-card__header) {
  background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
  border-bottom: 1px solid #e0e0e0;
}

:deep(.el-form-item__label) {
  font-weight: 500;
}

:deep(.el-input-number) {
  width: 100%;
}

.message-container {
  margin-top: 20px;
}

.zlm-section {
  margin-top: 20px;
}

.zlm-section :deep(.el-tabs__content) {
  padding: 20px 0;
}

.zlm-section :deep(.el-form-item) {
  margin-bottom: 18px;
}

.zlm-section :deep(.el-form-item__label) {
  font-weight: 500;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  line-height: 1.4;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    gap: 15px;
    align-items: flex-start;
  }
  
  .header-actions {
    width: 100%;
    justify-content: flex-end;
  }
  
  .settings-container {
    padding: 15px;
  }
}

@media (max-width: 576px) {
  .page-header h1 {
    font-size: 20px;
  }
  
  :deep(.el-form-item__label) {
    width: 100px !important;
  }
}
</style>