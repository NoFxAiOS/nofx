FROM python:3.10-slim

WORKDIR /app

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

# 复制 Space 文件
COPY requirements.txt mkdocs.yml docs/index.md /app/

# 安装 Python 依赖
RUN pip install --no-cache-dir -r requirements.txt

# 拉取 GitHub 仓库中文档
RUN mkdir -p docs/i18n/zh-CN
RUN git clone --depth 1 https://github.com/NoFxAiOS/nofx.git temp_repo && \
    cp -r temp_repo/docs/i18n/zh-CN/README.md docs/i18n/zh-CN/README.md && \
    rm -rf temp_repo

# 暴露端口
EXPOSE 7860

# 启动 MkDocs 文档站
CMD ["mkdocs", "serve", "-a", "0.0.0.0:7860"]
