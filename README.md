site_name: NoFx 中文文档
site_description: NoFx 中文文档站
theme:
  name: material
  palette:
    primary: blue
    accent: green
nav:
  - 首页: index.md
  - 文档:
      - 中文: i18n/zh-CN/README.md
markdown_extensions:
  - toc:
      permalink: true
  - admonition
  - codehilite
  - footnotes
  - pymdownx.superfences
