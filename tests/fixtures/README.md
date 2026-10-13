测试图像由项目创建，可按项目 MIT 许可使用。primary-multiple.heic 包含两幅纯色图像，主图为 320×180 红色；第二幅为 50×100 绿色，用于确认解码主图。orientation-6.jpg 为 40×80、EXIF 方向 6 的绿色图，用于确认旋转后为 80×40。primary.png 带 sRGB 配置用于颜色验证。

orientation-6.heic 通过 libheif 官方编码 API 的 image_orientation=6 生成 irot 变换，绿色图应显示为 80×40，用于验证 HEIC 自身方向属性。
