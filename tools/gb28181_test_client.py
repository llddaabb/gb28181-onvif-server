#!/usr/bin/env python3
"""
GB28181 多通道测试客户端
模拟 GB28181 设备（带多个 RTSP 通道）注册到 SIP 服务器

用法:
    python3 gb28181_test_client.py [--server-ip IP] [--server-port PORT] [--channels N]

示例:
    python3 gb28181_test_client.py --server-ip 192.168.1.100
    python3 gb28181_test_client.py --channels 4
"""

import socket
import hashlib
import random
import time
import sys
import threading
import argparse
import re
import subprocess
import shutil
from datetime import datetime

# 公共测试 RTSP 流地址列表（2024年验证可用）
PUBLIC_RTSP_STREAMS = [
    # RTSP.stream 官方测试流（稳定可用）
    "rtmp://ns8.indexforce.com/home/mystream",
    "rtmp://ns8.indexforce.com/home/mystream", 
    # Wowza 官方测试流
    "rtmp://ns8.indexforce.com/home/mystream",
    # 其他公共测试流
    "rtmp://ns8.indexforce.com/home/mystream",
    # 如果以上不可用，可以使用本地测试流
    # FFmpeg生成测试流命令: 
    # ffmpeg -re -f lavfi -i testsrc=size=640x480:rate=25 -f lavfi -i sine=frequency=1000 -c:v libx264 -preset ultrafast -tune zerolatency -c:a aac -f rtsp rtsp://localhost:8554/test
]


class GB28181MultiChannelClient:
    def __init__(self, server_ip, server_port=5060, num_channels=4, device_id=None):
        self.server_ip = server_ip
        self.server_port = server_port
        
        # 设备配置
        self.device_id = device_id or "34020000001320000001"  # 20位设备编码
        self.server_id = "34020000002000000001"  # 服务器ID
        self.realm = "3402000000"
        self.password = ""  # SIP密码（如果服务器需要）
        
        # 本地配置
        self.local_ip = self._get_local_ip()
        self.local_port = random.randint(5080, 5099)
        
        # SIP 参数
        self.call_id = self._generate_call_id()
        self.cseq = 1
        self.tag = self._generate_tag()
        self.branch = self._generate_branch()
        
        # 生成多通道信息
        self.channels = self._generate_channels(num_channels)
        
        # Socket
        self.sock = None
        self.running = False
        
        # 活动会话（用于跟踪 INVITE）
        self.active_sessions = {}
        
        # FFmpeg 进程管理
        self.ffmpeg_processes = {}
        
    def _get_local_ip(self):
        """获取本机IP"""
        try:
            s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            s.connect((self.server_ip, 80))
            ip = s.getsockname()[0]
            s.close()
            return ip
        except:
            return "127.0.0.1"
    
    def _generate_channels(self, num_channels):
        """生成多通道信息"""
        channels = []
        base_id = self.device_id[:14]  # 取前14位作为基础
        
        channel_names = [
            "大门入口", "停车场A区", "电梯间1", "办公大厅",
            "后门通道", "停车场B区", "电梯间2", "会议室",
            "前台", "仓库", "楼梯间", "天台",
            "地下室", "机房", "消防通道", "值班室"
        ]
        
        for i in range(num_channels):
            channel_id = f"{base_id}{(132 + i):06d}"  # 生成通道编码
            rtsp_url = PUBLIC_RTSP_STREAMS[i % len(PUBLIC_RTSP_STREAMS)]
            
            channels.append({
                "id": channel_id,
                "name": channel_names[i % len(channel_names)] if i < len(channel_names) else f"通道{i+1}",
                "manufacturer": "TestDevice",
                "model": f"IPC-{1000 + i}",
                "status": "ON",
                "rtsp_url": rtsp_url,
                "ptz_type": 1 if i % 2 == 0 else 0,  # 部分支持PTZ
            })
            
        return channels
    
    def _generate_call_id(self):
        """生成 Call-ID"""
        return f"{random.randint(100000, 999999)}@{self.local_ip}"
    
    def _generate_tag(self):
        """生成 Tag"""
        return f"{random.randint(100000, 999999)}"
    
    def _generate_branch(self):
        """生成 Branch"""
        return f"z9hG4bK{random.randint(100000, 999999)}"
    
    def _build_register(self, auth_header=None):
        """构建 REGISTER 请求"""
        self.cseq += 1
        self.branch = self._generate_branch()
        
        headers = [
            f"REGISTER sip:{self.server_id}@{self.server_ip}:{self.server_port} SIP/2.0",
            f"Via: SIP/2.0/UDP {self.local_ip}:{self.local_port};rport;branch={self.branch}",
            f"From: <sip:{self.device_id}@{self.realm}>;tag={self.tag}",
            f"To: <sip:{self.device_id}@{self.realm}>",
            f"Call-ID: {self.call_id}",
            f"CSeq: {self.cseq} REGISTER",
            f"Contact: <sip:{self.device_id}@{self.local_ip}:{self.local_port}>",
            f"Max-Forwards: 70",
            f"User-Agent: GB28181-MultiChannel-TestClient/1.0",
            f"Expires: 3600",
        ]
        
        if auth_header:
            headers.append(auth_header)
        
        headers.append("Content-Length: 0")
        headers.append("")
        headers.append("")
        
        return "\r\n".join(headers)
    
    def _build_catalog_response(self, from_tag, to_tag, call_id, cseq):
        """构建目录查询响应"""
        # 构建设备目录 XML
        device_list = ""
        for ch in self.channels:
            device_list += f"""
        <Item>
            <DeviceID>{ch['id']}</DeviceID>
            <Name>{ch['name']}</Name>
            <Manufacturer>{ch['manufacturer']}</Manufacturer>
            <Model>{ch['model']}</Model>
            <Owner>Owner</Owner>
            <CivilCode>3402000000</CivilCode>
            <Address>TestAddress</Address>
            <Parental>0</Parental>
            <SafetyWay>0</SafetyWay>
            <RegisterWay>1</RegisterWay>
            <Secrecy>0</Secrecy>
            <Status>{ch['status']}</Status>
            <PTZType>{ch.get('ptz_type', 0)}</PTZType>
        </Item>"""
        
        # 使用 UTF-8 编码而不是 GB2312，便于服务器解析
        xml_body = f"""<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <CmdType>Catalog</CmdType>
    <SN>1</SN>
    <DeviceID>{self.device_id}</DeviceID>
    <SumNum>{len(self.channels)}</SumNum>
    <DeviceList Num="{len(self.channels)}">{device_list}
    </DeviceList>
</Response>"""
        
        body = xml_body.encode('utf-8')
        
        self.cseq += 1
        headers = [
            f"MESSAGE sip:{self.server_id}@{self.server_ip}:{self.server_port} SIP/2.0",
            f"Via: SIP/2.0/UDP {self.local_ip}:{self.local_port};rport;branch={self._generate_branch()}",
            f"From: <sip:{self.device_id}@{self.realm}>;tag={self.tag}",
            f"To: <sip:{self.server_id}@{self.realm}>",
            f"Call-ID: {self._generate_call_id()}",
            f"CSeq: {self.cseq} MESSAGE",
            f"Max-Forwards: 70",
            f"User-Agent: GB28181-MultiChannel-TestClient/1.0",
            f"Content-Type: Application/MANSCDP+xml",
            f"Content-Length: {len(body)}",
        ]
        
        # 正确构建SIP消息：headers + 空行 + body
        message = "\r\n".join(headers) + "\r\n\r\n"
        return message.encode() + body
    
    def _build_keepalive(self):
        """构建心跳 MESSAGE"""
        self.cseq += 1
        
        xml_body = f"""<?xml version="1.0" encoding="UTF-8"?>
<Notify>
    <CmdType>Keepalive</CmdType>
    <SN>{int(time.time())}</SN>
    <DeviceID>{self.device_id}</DeviceID>
    <Status>OK</Status>
</Notify>"""
        
        body = xml_body.encode('utf-8')
        
        headers = [
            f"MESSAGE sip:{self.server_id}@{self.server_ip}:{self.server_port} SIP/2.0",
            f"Via: SIP/2.0/UDP {self.local_ip}:{self.local_port};rport;branch={self._generate_branch()}",
            f"From: <sip:{self.device_id}@{self.realm}>;tag={self.tag}",
            f"To: <sip:{self.server_id}@{self.realm}>",
            f"Call-ID: {self._generate_call_id()}",
            f"CSeq: {self.cseq} MESSAGE",
            f"Max-Forwards: 70",
            f"User-Agent: GB28181-MultiChannel-TestClient/1.0",
            f"Content-Type: Application/MANSCDP+xml",
            f"Content-Length: {len(body)}",
        ]
        
        # 正确构建SIP消息：headers + 空行 + body
        message = "\r\n".join(headers) + "\r\n\r\n"
        return message.encode() + body
    
    def _build_200_ok(self, request_text):
        """构建 200 OK 响应"""
        lines = request_text.split('\r\n')
        
        # 提取必要的头部
        via = ""
        from_header = ""
        to_header = ""
        call_id = ""
        cseq = ""
        
        for line in lines:
            lower = line.lower()
            if lower.startswith('via:'):
                via = line
            elif lower.startswith('from:'):
                from_header = line
            elif lower.startswith('to:'):
                to_header = line
                # 如果 To 没有 tag，添加一个
                if 'tag=' not in to_header:
                    to_header += f";tag={self._generate_tag()}"
            elif lower.startswith('call-id:'):
                call_id = line
            elif lower.startswith('cseq:'):
                cseq = line
        
        headers = [
            "SIP/2.0 200 OK",
            via,
            from_header,
            to_header,
            call_id,
            cseq,
            f"User-Agent: GB28181-MultiChannel-TestClient/1.0",
            "Content-Length: 0",
            "",
            "",
        ]
        
        return "\r\n".join(headers)
    
    def _build_invite_response(self, request_text, channel_id):
        """构建 INVITE 200 OK 响应（带 SDP）"""
        lines = request_text.split('\r\n')
        
        # 提取必要的头部
        via = ""
        from_header = ""
        to_header = ""
        call_id_header = ""
        cseq = ""
        
        for line in lines:
            lower = line.lower()
            if lower.startswith('via:'):
                via = line
            elif lower.startswith('from:'):
                from_header = line
            elif lower.startswith('to:'):
                to_header = line
                if 'tag=' not in to_header:
                    to_header += f";tag={self._generate_tag()}"
            elif lower.startswith('call-id:'):
                call_id_header = line
            elif lower.startswith('cseq:'):
                cseq = line
        
        # 获取通道的 RTSP 流地址
        channel = None
        for ch in self.channels:
            if ch['id'] == channel_id:
                channel = ch
                break
        
        rtsp_url = channel['rtsp_url'] if channel else PUBLIC_RTSP_STREAMS[0]
        
        # 构建 SDP（这里返回 RTSP 流信息）
        # 实际 GB28181 使用 RTP/PS 流，这里简化为指示 RTSP 源
        sdp = f"""v=0
o=- {int(time.time())} 1 IN IP4 {self.local_ip}
s=Play
c=IN IP4 {self.local_ip}
t=0 0
m=video 0 RTP/AVP 96
a=rtpmap:96 PS/90000
a=sendonly
a=control:{rtsp_url}
y=0000000001
f=v/2/4/25/1/4000a///
"""
        
        body = sdp.encode()
        
        headers = [
            "SIP/2.0 200 OK",
            via,
            from_header,
            to_header,
            call_id_header,
            cseq,
            f"Contact: <sip:{channel_id}@{self.local_ip}:{self.local_port}>",
            f"User-Agent: GB28181-MultiChannel-TestClient/1.0",
            "Content-Type: application/sdp",
            f"Content-Length: {len(body)}",
            "",
        ]
        
        return "\r\n".join(headers).encode() + body
    
    def _parse_response(self, data):
        """解析 SIP 响应/请求"""
        text = data.decode('utf-8', errors='ignore')
        lines = text.split('\r\n')
        if not lines:
            return None, None, {}, text
        
        # 解析状态行
        status_line = lines[0]
        parts = status_line.split(' ', 2)
        
        if len(parts) >= 2:
            if parts[0].startswith('SIP'):
                code = int(parts[1])
                message = parts[2] if len(parts) > 2 else ""
            else:
                # 这是请求，返回方法名
                code = None
                message = parts[0]
        else:
            return None, None, {}, text
        
        # 解析头部
        headers = {}
        for line in lines[1:]:
            if ':' in line:
                key, value = line.split(':', 1)
                headers[key.strip().lower()] = value.strip()
        
        return code, message, headers, text
    
    def _extract_channel_from_invite(self, text):
        """从 INVITE 请求中提取通道 ID"""
        # 尝试从 Request-URI 提取
        match = re.search(r'INVITE sip:(\d{20})@', text)
        if match:
            return match.group(1)
        
        # 尝试从 To 头提取
        match = re.search(r'To:.*?<sip:(\d{20})@', text)
        if match:
            return match.group(1)
        
        return self.channels[0]['id'] if self.channels else None
    
    def _extract_rtp_info_from_sdp(self, text):
        """从 INVITE SDP 中提取 RTP 目标地址和端口"""
        # 提取 IP 地址 (c= 行)
        ip_match = re.search(r'c=IN IP4 (\d+\.\d+\.\d+\.\d+)', text)
        rtp_ip = ip_match.group(1) if ip_match else self.server_ip
        
        # 提取 RTP 端口 (m=video 行)
        port_match = re.search(r'm=video (\d+)', text)
        rtp_port = int(port_match.group(1)) if port_match else 0
        
        # 提取 SSRC (y= 行, GB28181 扩展)
        ssrc_match = re.search(r'y=(\d+)', text)
        ssrc = ssrc_match.group(1) if ssrc_match else "0000000001"
        
        return rtp_ip, rtp_port, ssrc
    
    def _start_ffmpeg_stream(self, channel_id, channel, rtp_ip, rtp_port, ssrc):
        """启动 FFmpeg 推流到 ZLM RTP 端口"""
        # 检查 FFmpeg 是否可用
        ffmpeg_path = shutil.which('ffmpeg')
        if not ffmpeg_path:
            print("    [警告] FFmpeg 未安装，无法推流")
            return None
        
        rtmp_url = channel['rtsp_url']
        
        # 构建 FFmpeg 命令推送 PS 流到 RTP
        # GB28181 使用 PS (Program Stream) 封装
        ffmpeg_cmd = [
            'ffmpeg',
            '-re',                          # 实时读取
            '-i', rtmp_url,                 # 输入源
            '-c:v', 'libx264',              # H264 编码
            '-preset', 'ultrafast',         # 最快编码
            '-tune', 'zerolatency',         # 低延迟
            '-profile:v', 'baseline',       # 兼容性
            '-b:v', '1000k',                # 码率
            '-an',                          # 不要音频
            '-f', 'rtp_mpegts',             # RTP MPEG-TS 封装
            f'rtp://{rtp_ip}:{rtp_port}?localport={random.randint(20000, 30000)}'
        ]
        
        print(f"    [推流] 启动 FFmpeg 推流...")
        print(f"    输入: {rtmp_url}")
        print(f"    输出: rtp://{rtp_ip}:{rtp_port}")
        
        try:
            # 启动 FFmpeg 进程（后台运行）
            process = subprocess.Popen(
                ffmpeg_cmd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.PIPE,
                stdin=subprocess.DEVNULL
            )
            
            # 等待一下看是否有错误
            time.sleep(0.5)
            if process.poll() is not None:
                stderr = process.stderr.read().decode('utf-8', errors='ignore')
                print(f"    [错误] FFmpeg 启动失败: {stderr[:200]}")
                return None
            
            # 保存进程
            self.ffmpeg_processes[channel_id] = process
            print(f"    [成功] FFmpeg 推流已启动 (PID: {process.pid})")
            return process
            
        except Exception as e:
            print(f"    [错误] 启动 FFmpeg 失败: {e}")
            return None
    
    def _stop_ffmpeg_stream(self, channel_id):
        """停止 FFmpeg 推流"""
        if channel_id in self.ffmpeg_processes:
            process = self.ffmpeg_processes[channel_id]
            if process and process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=3)
                except:
                    process.kill()
                print(f"    [停止] FFmpeg 推流已停止 (通道: {channel_id})")
            del self.ffmpeg_processes[channel_id]
    
    def _stop_all_ffmpeg(self):
        """停止所有 FFmpeg 推流"""
        for channel_id in list(self.ffmpeg_processes.keys()):
            self._stop_ffmpeg_stream(channel_id)
    
    def _send(self, message):
        """发送 SIP 消息"""
        if isinstance(message, str):
            message = message.encode()
        self.sock.sendto(message, (self.server_ip, self.server_port))
        print(f"[发送] -> {self.server_ip}:{self.server_port}")
    
    def _receive(self, timeout=5):
        """接收 SIP 消息"""
        self.sock.settimeout(timeout)
        try:
            data, addr = self.sock.recvfrom(65535)
            print(f"[接收] <- {addr[0]}:{addr[1]} ({len(data)} bytes)")
            return data, addr
        except socket.timeout:
            return None, None
    
    def register(self):
        """执行注册流程"""
        print(f"\n{'='*70}")
        print(f"GB28181 多通道测试客户端")
        print(f"{'='*70}")
        print(f"服务器: {self.server_ip}:{self.server_port}")
        print(f"设备ID: {self.device_id}")
        print(f"本地地址: {self.local_ip}:{self.local_port}")
        print(f"通道数量: {len(self.channels)}")
        print(f"{'='*70}")
        print(f"\n通道列表:")
        for i, ch in enumerate(self.channels):
            print(f"  {i+1}. {ch['id']} - {ch['name']}")
            print(f"     RTSP: {ch['rtsp_url']}")
        print(f"{'='*70}\n")
        
        # 创建 UDP Socket
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.sock.bind(('0.0.0.0', self.local_port))
        
        try:
            # 第一次 REGISTER（可能收到 401）
            print("[1] 发送 REGISTER 请求...")
            self._send(self._build_register())
            
            data, addr = self._receive()
            if not data:
                print("[错误] 服务器无响应")
                return False
            
            code, message, headers, _ = self._parse_response(data)
            print(f"    收到响应: {code} {message}")
            
            if code == 401:
                # 需要认证
                print("[2] 服务器要求认证，计算认证响应...")
                www_auth = headers.get('www-authenticate', '')
                auth_header = self._build_auth_header(www_auth)
                
                self._send(self._build_register(auth_header))
                data, addr = self._receive()
                if data:
                    code, message, headers, _ = self._parse_response(data)
                    print(f"    收到响应: {code} {message}")
            
            if code == 200:
                print("\n[成功] 设备注册成功!")
                self.running = True
                return True
            else:
                print(f"\n[失败] 注册失败: {code} {message}")
                return False
                
        except Exception as e:
            print(f"[错误] {e}")
            import traceback
            traceback.print_exc()
            return False
    
    def _build_auth_header(self, www_auth):
        """构建认证头（简化版，实际需要完整的 Digest 认证）"""
        # 提取 realm 和 nonce
        realm_match = re.search(r'realm="([^"]*)"', www_auth)
        nonce_match = re.search(r'nonce="([^"]*)"', www_auth)
        
        realm = realm_match.group(1) if realm_match else self.realm
        nonce = nonce_match.group(1) if nonce_match else ""
        
        # 计算响应（简化版）
        uri = f"sip:{self.server_id}@{self.server_ip}:{self.server_port}"
        
        # HA1 = MD5(username:realm:password)
        ha1 = hashlib.md5(f"{self.device_id}:{realm}:{self.password}".encode()).hexdigest()
        # HA2 = MD5(method:uri)
        ha2 = hashlib.md5(f"REGISTER:{uri}".encode()).hexdigest()
        # Response = MD5(HA1:nonce:HA2)
        response = hashlib.md5(f"{ha1}:{nonce}:{ha2}".encode()).hexdigest()
        
        return f'Authorization: Digest username="{self.device_id}", realm="{realm}", nonce="{nonce}", uri="{uri}", response="{response}", algorithm=MD5'
    
    def start_heartbeat(self):
        """启动心跳线程"""
        def heartbeat_loop():
            while self.running:
                time.sleep(30)  # 每30秒发送心跳
                if not self.running:
                    break
                try:
                    print(f"\n[心跳] 发送 Keepalive @ {datetime.now().strftime('%H:%M:%S')}")
                    self._send(self._build_keepalive())
                    data, addr = self._receive(timeout=5)
                    if data:
                        code, _, _, _ = self._parse_response(data)
                        print(f"    心跳响应: {code}")
                except Exception as e:
                    print(f"[心跳错误] {e}")
        
        thread = threading.Thread(target=heartbeat_loop, daemon=True)
        thread.start()
        return thread
    
    def listen_for_commands(self):
        """监听服务器命令（如目录查询、点播等）"""
        print("\n[监听] 等待服务器命令...")
        print("提示: 可以在前端页面进行以下操作来测试:")
        print("  - 点击 '刷新通道' 触发目录查询")
        print("  - 点击通道的 '点播' 按钮触发 INVITE")
        print("按 Ctrl+C 停止\n")
        
        self.sock.settimeout(1)
        
        while self.running:
            try:
                data, addr = self.sock.recvfrom(65535)
                print(f"\n{'='*60}")
                print(f"[收到命令] <- {addr[0]}:{addr[1]} @ {datetime.now().strftime('%H:%M:%S')}")
                
                code, method, headers, text = self._parse_response(data)
                
                if code is not None:
                    # 这是响应，不需要处理
                    print(f"    响应: {code}")
                    continue
                
                print(f"    方法: {method}")
                
                if method == "MESSAGE":
                    # 检查是否是目录查询
                    if 'Catalog' in text:
                        print("    命令类型: 目录查询 (Catalog)")
                        print(f"    发送设备目录 ({len(self.channels)} 个通道)...")
                        
                        # 先发送 200 OK
                        ok_response = self._build_200_ok(text)
                        self.sock.sendto(ok_response.encode(), addr)
                        print("    已发送 200 OK")
                        
                        # 发送目录响应
                        time.sleep(0.1)
                        catalog_response = self._build_catalog_response(
                            self.tag,
                            self._generate_tag(),
                            headers.get('call-id', ''),
                            1
                        )
                        self._send(catalog_response)
                        print("    已发送目录响应")
                        
                    elif 'Keepalive' in text:
                        print("    命令类型: 心跳响应")
                        
                    else:
                        print("    命令类型: 其他 MESSAGE")
                        # 发送 200 OK
                        ok_response = self._build_200_ok(text)
                        self.sock.sendto(ok_response.encode(), addr)
                        
                elif method == "INVITE":
                    print("    命令类型: 视频点播 (INVITE)")
                    channel_id = self._extract_channel_from_invite(text)
                    print(f"    请求通道: {channel_id}")
                    
                    # 提取 RTP 目标信息
                    rtp_ip, rtp_port, ssrc = self._extract_rtp_info_from_sdp(text)
                    print(f"    RTP目标: {rtp_ip}:{rtp_port} (SSRC: {ssrc})")
                    
                    # 查找通道
                    channel = None
                    for ch in self.channels:
                        if ch['id'] == channel_id:
                            channel = ch
                            break
                    
                    if channel:
                        print(f"    通道名称: {channel['name']}")
                        print(f"    流源: {channel['rtsp_url']}")
                        
                        # 发送 100 Trying
                        trying_resp = self._build_100_trying(text)
                        self.sock.sendto(trying_resp.encode(), addr)
                        print("    已发送 100 Trying")
                        
                        time.sleep(0.1)
                        
                        # 发送 200 OK (带 SDP)
                        ok_resp = self._build_invite_response(text, channel_id)
                        self.sock.sendto(ok_resp, addr)
                        print("    已发送 200 OK (带 SDP)")
                        
                        # 启动 FFmpeg 推流
                        if rtp_port > 0:
                            self._start_ffmpeg_stream(channel_id, channel, rtp_ip, rtp_port, ssrc)
                        
                        # 记录会话
                        call_id = headers.get('call-id', '')
                        self.active_sessions[call_id] = {
                            'channel_id': channel_id,
                            'channel': channel,
                            'start_time': time.time()
                        }
                    else:
                        print(f"    [警告] 未找到通道 {channel_id}")
                        
                elif method == "BYE":
                    print("    命令类型: 挂断 (BYE)")
                    # 发送 200 OK
                    ok_response = self._build_200_ok(text)
                    self.sock.sendto(ok_response.encode(), addr)
                    print("    已发送 200 OK")
                    
                    # 清理会话，停止推流
                    call_id = headers.get('call-id', '')
                    if call_id in self.active_sessions:
                        session = self.active_sessions[call_id]
                        self._stop_ffmpeg_stream(session['channel_id'])
                        del self.active_sessions[call_id]
                        print("    会话已清理，推流已停止")
                        
                elif method == "ACK":
                    print("    命令类型: 确认 (ACK)")
                    # ACK 不需要响应
                    
                else:
                    print(f"    未知命令: {method}")
                    # 发送 200 OK
                    ok_response = self._build_200_ok(text)
                    self.sock.sendto(ok_response.encode(), addr)
                
                print(f"{'='*60}")
                
            except socket.timeout:
                continue
            except Exception as e:
                if self.running:
                    print(f"[错误] {e}")
                    import traceback
                    traceback.print_exc()
    
    def _build_100_trying(self, request_text):
        """构建 100 Trying 响应"""
        lines = request_text.split('\r\n')
        
        via = ""
        from_header = ""
        to_header = ""
        call_id = ""
        cseq = ""
        
        for line in lines:
            lower = line.lower()
            if lower.startswith('via:'):
                via = line
            elif lower.startswith('from:'):
                from_header = line
            elif lower.startswith('to:'):
                to_header = line
            elif lower.startswith('call-id:'):
                call_id = line
            elif lower.startswith('cseq:'):
                cseq = line
        
        headers = [
            "SIP/2.0 100 Trying",
            via,
            from_header,
            to_header,
            call_id,
            cseq,
            "Content-Length: 0",
            "",
            "",
        ]
        
        return "\r\n".join(headers)
    
    def stop(self):
        """停止客户端"""
        self.running = False
        # 停止所有 FFmpeg 推流
        self._stop_all_ffmpeg()
        if self.sock:
            self.sock.close()
        print("\n[停止] 客户端已停止")


def main():
    parser = argparse.ArgumentParser(description='GB28181 多通道测试客户端')
    parser.add_argument('--server-ip', '-s', default='127.0.0.1', help='GB28181 服务器 IP 地址')
    parser.add_argument('--server-port', '-p', type=int, default=5060, help='SIP 端口 (默认: 5060)')
    parser.add_argument('--channels', '-c', type=int, default=4, help='通道数量 (默认: 4)')
    parser.add_argument('--device-id', '-d', default=None, help='设备 ID (默认自动生成)')
    
    args = parser.parse_args()
    
    client = GB28181MultiChannelClient(
        server_ip=args.server_ip,
        server_port=args.server_port,
        num_channels=args.channels,
        device_id=args.device_id
    )
    
    try:
        if client.register():
            # 启动心跳
            client.start_heartbeat()
            
            # 监听服务器命令
            client.listen_for_commands()
    except KeyboardInterrupt:
        print("\n\n收到中断信号...")
    finally:
        client.stop()


if __name__ == "__main__":
    main()
