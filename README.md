### Fus
```shell
# 项目结构：
backend/
├── config/
│   └── config.go        # 配置加载
├── handlers/
│   ├── auth.go          # 认证处理
│   ├── file.go          # 文件处理
│   ├── upload.go        # 上传处理
│   └── directory.go     # 目录处理
├── middleware/
│   └── auth.go          # 认证中间件
├── utils/
│   └── fileutils.go     # 文件工具函数
├── cleanup/
│   └── cleanup.go       # 清理任务
├── .env                 # 环境变量
└── main.go              # 程序入口

frontend/
├── templates/
│   ├── login.html       # 登录页面
│   └── index.html       # 首页
└── static/
    └── style.css        # CSS样式

storage/
├── └── data/            # 上传文件存放位置
```
### Docker
```
# Docker部署：
git clone https://github.com/zhanghao123321/Fus.git
cd Fus
docker build -t fus-app:latest .
docker run -d  --name fus -p 8888:8080 
     -e PORT=8080 \
     -e AUTH_USERS="admin:admin,zz:zz" \
     -v /data/storage:/storage/data \
     fus-app:latest
```

