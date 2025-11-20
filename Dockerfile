FROM python:3.11-slim

WORKDIR /app

# 复制依赖文件
COPY requirements.txt .

# 安装依赖
RUN pip install --no-cache-dir -r requirements.txt

# 复制应用代码
COPY app.py .

# 暴露端口
EXPOSE 7860

# 启动应用
CMD ["streamlit", "run", "app.py", "--server.port=7860", "--server.address=0.0.0.0"]
