FROM python:3.11-slim

WORKDIR /app

# 复制依赖文件
COPY requirements.txt .

# 安装依赖
RUN pip install --no-cache-dir -r requirements.txt

# 复制应用代码
COPY . .

# 创建非root用户
RUN useradd -m -u 1000 user
USER user

# 暴露端口
EXPOSE 5000

# 启动应用
CMD ["gunicorn", "--bind", "0.0.0.0:5000", "app:app"]
