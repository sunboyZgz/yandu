---
title: URL 与目录映射
description: 区分页面基础路径、资源前缀和磁盘根目录，准确预测实际读取的文件。
---

# URL 与目录映射

最容易混淆的是“哪些请求进入资源链路”和“读文件时去掉哪段路径”。这由两个不同字段控制。

## 三个字段的关系

```text
本地素材目录：  D:/oss/blog
页面基础路径：  /blog
资源路径范围：  /blog/xxx/
```

对于 `/blog/xxx/xxxxx/xx.png`：

```text
1. 范围匹配    /blog/xxx/xxxxx/xx.png 属于 /blog/xxx/
2. 去掉基础    /blog
3. 保留余下    xxx/xxxxx/xx.png
4. 读取文件    D:/oss/blog/xxx/xxxxx/xx.png
```

**只去掉一次 `/blog`，不会去掉整个 `/blog/xxx/`。** 云端和 frp 保留请求路径；本地文件服务完成转换。

## 预期结果

| 请求 | 结果 |
| --- | --- |
| `/blog`、`/blog/` | 原云端页面 |
| `/blog/posts/123` | 原云端文章 |
| `/blog/api/users` | 原业务 API |
| `/blog/xxx/a.png` | `D:/oss/blog/xxx/a.png` |
| `/blog/xxxevil/a.png` | 不命中 `/blog/xxx/`，继续原网站行为 |
| `/blog/xxx/missing.png` | 资源 404，不交回 SPA 首页 |

停用的资源范围继续保留拒绝规则。只有独立“释放给网站”后，原网站才重新接管这个范围。

## 路径写法

- 配置路径使用字母、数字、下划线和连字符组成的 ASCII 段。
- 页面基础路径不以 `/` 结尾，例如 `/blog`。
- 资源前缀以 `/` 结尾，而且比基础路径更深，例如 `/blog/xxx/`。
- 不填写通配符 `*`、正则、域名或 Nginx 语句。
- 资源范围必须在连接包和云端共同授予的路径授权内。

`/blog/api/`、`assets/`、`posts/`、`_next/` 等业务范围受保护，不能整体映射为本地素材。

## 中文与空格文件名

叶子文件名可以使用中文和空格，URL 按段编码：

```text
磁盘：D:/oss/blog/xxx/中文 图片.png
URL：https://example.com/blog/xxx/%E4%B8%AD%E6%96%87%20%E5%9B%BE%E7%89%87.png
```

在“目录与文件”中复制资源地址，可获得正确编码的 URL。查询串不用于拼接磁盘路径，工具也不会把它当作现成的业务签名授权。

点段、反斜杠、编码斜杠、重复编码、NUL、Windows ADS/设备名以及尾部点或空格会被拒绝。Windows 的大小写和重解析点行为仍需结合目标环境验证。
