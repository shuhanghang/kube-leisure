# kube-leisure
定时`滚动`重启deployment或statefulset资源对象，等效于`kubectl rollout restart`

## 开始
1. 安装
```sh
    kubectl apply -f 
```
2. 部署自定义`重启负载`文件
```yaml
apiVersion: leisure.shuhanghang.com/v1beta1
kind: Leisure
metadata:
  name: leisure-sample
spec:
  restart:                     #重启资源
    resourceType: deployment   #资源类型
    name: nginx                #资源名称
    nameSpace: default         #资源所在的空间
    restartAt: '0 1 * * *'     #crontab格式
    timeZone: Asia/Shanghai    #所在时区
--- 
apiVersion: leisure.shuhanghang.com/v1beta1
kind: Leisure
metadata:
  name: leisure-sample2
spec:
  restart:
    resourceType: statefulset
    name: postgresql
    nameSpace: database
    restartAt: '0 2 1 * *'
    timeZone: Asia/Shanghai
```