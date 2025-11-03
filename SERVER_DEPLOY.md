# NOFX Trading Bot 服务器部署指南

## 🚀 快速部署

### 方案1: 使用优化后的代码（推荐）

1. **将本地代码上传到服务器**:
```bash
# 在本地打包代码
cd d:\Projects\nofx
tar -czf nofx-optimized.tar.gz . --exclude=node_modules --exclude=decision_logs

# 上传到服务器
scp nofx-optimized.tar.gz user@your-server:/tmp/
```

2. **在服务器上解压并部署**:
```bash
# SSH登录服务器
ssh user@your-server

# 解压代码
cd /opt
sudo mkdir nofx-trading
cd nofx-trading
sudo tar -xzf /tmp/nofx-optimized.tar.gz
sudo chown -R $USER:$USER .

# 运行部署脚本
chmod +x deploy-server.sh
./deploy-server.sh
```

### 方案2: Git仓库部署

1. **Fork原仓库到您的GitHub账户**
2. **推送优化分支**:
```bash
git remote add myfork git@github.com:YOUR_USERNAME/nofx.git
git push myfork strategy-optimization-v2
```

3. **在服务器上克隆**:
```bash
cd /opt
sudo git clone https://github.com/YOUR_USERNAME/nofx.git nofx-trading
cd nofx-trading
sudo git checkout strategy-optimization-v2
./deploy-server.sh
```

## ⚙️ 配置说明

### 1. 修改配置文件
```bash
sudo nano config.json
```

重要参数:
- `aster_private_key`: 您的交易私钥
- `deepseek_key`: 您的DeepSeek API密钥
- `initial_balance`: 初始资金
- `leverage`: 杠杆倍数设置

### 2. 环境变量（可选）
创建 `.env` 文件:
```bash
DEEPSEEK_API_KEY=your_deepseek_key
ASTER_PRIVATE_KEY=your_private_key
INITIAL_BALANCE=137.5
```

## 🔧 运维命令

### 启动/停止
```bash
cd /opt/nofx-trading

# 启动
sudo docker-compose up -d

# 停止
sudo docker-compose down

# 重启
sudo docker-compose restart

# 查看日志
sudo docker-compose logs -f
```

### 监控
```bash
# 查看容器状态
sudo docker-compose ps

# 查看资源使用
sudo docker stats

# 查看最新决策
tail -f decision_logs/aster_deepseek/decision_*.json
```

## � 生产环境增强

### 使用生产配置
```bash
# 复制生产环境配置
cp docker-compose.prod.yml /opt/nofx-trading/
cp env.server.example /opt/nofx-trading/.env

# 编辑环境变量
nano /opt/nofx-trading/.env
# 填入真实的 DEEPSEEK_API_KEY 和 ASTER_PRIVATE_KEY

# 使用生产配置启动
cd /opt/nofx-trading
docker-compose -f docker-compose.prod.yml up -d
```

### 健康监控
```bash
# 安装健康检查脚本
cp health-check.sh /opt/nofx-trading/
chmod +x /opt/nofx-trading/health-check.sh

# 设置定时检查（每5分钟）
echo "*/5 * * * * root /opt/nofx-trading/health-check.sh" >> /etc/crontab

# 手动运行检查
sudo /opt/nofx-trading/health-check.sh
```

### 自动备份恢复
```bash
# 安装备份脚本
cp backup-restore.sh /opt/nofx-trading/
chmod +x /opt/nofx-trading/backup-restore.sh

# 创建备份
sudo /opt/nofx-trading/backup-restore.sh backup

# 设置每日自动备份
echo "0 2 * * * root /opt/nofx-trading/backup-restore.sh backup" >> /etc/crontab

# 查看备份
sudo /opt/nofx-trading/backup-restore.sh list
```

## �🛡️ 安全建议

1. **防火墙设置**:
```bash
sudo ufw allow 22    # SSH
sudo ufw allow 3000  # 前端（可选，内网访问）
sudo ufw allow 8080  # API（可选，内网访问）
sudo ufw enable
```

2. **SSL证书**（如果需要HTTPS）:
```bash
# 使用Let's Encrypt
sudo apt install certbot
sudo certbot --nginx -d yourdomain.com
```

3. **环境变量安全**:
```bash
# 设置正确的文件权限
chmod 600 /opt/nofx-trading/.env
chown root:root /opt/nofx-trading/.env
```

## 📊 监控面板

访问地址:
- 前端监控: http://your-server-ip:3000
- API接口: http://your-server-ip:8080/api/status

## 🆘 故障排除

### 常见问题

1. **容器启动失败**:
```bash
# 查看详细错误
sudo docker-compose logs

# 检查端口占用
sudo netstat -tlnp | grep :8080
```

2. **API密钥错误**:
```bash
# 检查配置文件
cat config.json | grep -E "(deepseek_key|aster_private_key)"
```

3. **内存不足**:
```bash
# 查看内存使用
free -h
# 增加swap
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

## 🔄 更新代码

```bash
cd /opt/nofx-trading

# 备份当前配置
cp config.json config.json.backup

# 拉取最新代码
git pull origin strategy-optimization-v2

# 重新构建
sudo docker-compose down
sudo docker-compose up -d --build

# 恢复配置
cp config.json.backup config.json
sudo docker-compose restart
```

## 📞 支持

如果遇到问题，请检查:
1. Docker容器日志
2. 系统资源使用情况
3. 网络连接状态
4. API密钥有效性