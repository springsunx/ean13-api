// EAN-13 条码解码器前端逻辑
(function() {
    'use strict';

    const uploadArea = document.getElementById('uploadArea');
    const fileInput = document.getElementById('fileInput');
    const previewArea = document.getElementById('previewArea');
    const previewImage = document.getElementById('previewImage');
    const btnClear = document.getElementById('btnClear');
    const btnDecode = document.getElementById('btnDecode');
    const resultArea = document.getElementById('resultArea');
    const resultText = document.getElementById('resultText');
    const resultFormat = document.getElementById('resultFormat');
    const btnCopy = document.getElementById('btnCopy');
    const errorArea = document.getElementById('errorArea');
    const errorText = document.getElementById('errorText');

    let currentFile = null;
    let currentObjectURL = null;
    let serverConfig = null;

    // 启动时获取服务器配置
    fetch('/api/config')
        .then(function(r) { return r.json(); })
        .then(function(cfg) { serverConfig = cfg; })
        .catch(function() {});

    // 点击上传区域触发文件选择
    uploadArea.addEventListener('click', function() {
        fileInput.click();
    });

    // 文件选择变化
    fileInput.addEventListener('change', function(e) {
        if (e.target.files && e.target.files[0]) {
            handleFile(e.target.files[0]);
        }
    });

    // 拖拽事件
    uploadArea.addEventListener('dragover', function(e) {
        e.preventDefault();
        uploadArea.classList.add('dragover');
    });

    uploadArea.addEventListener('dragleave', function(e) {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
    });

    uploadArea.addEventListener('drop', function(e) {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
        if (e.dataTransfer.files && e.dataTransfer.files[0]) {
            handleFile(e.dataTransfer.files[0]);
        }
    });

    // Ctrl+V 粘贴图片
    document.addEventListener('paste', function(e) {
        const items = e.clipboardData && e.clipboardData.items;
        if (!items) return;

        for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') !== -1) {
                const file = items[i].getAsFile();
                if (file) {
                    handleFile(file);
                    e.preventDefault();
                    break;
                }
            }
        }
    });

    // 清除图片
    btnClear.addEventListener('click', function(e) {
        e.stopPropagation();
        clearImage();
    });

    // 解码按钮
    btnDecode.addEventListener('click', function() {
        if (currentFile) {
            decodeImage(currentFile);
        }
    });

    // 复制按钮
    btnCopy.addEventListener('click', function() {
        const text = resultText.textContent;
        if (!text) return;

        // 使用 textarea + execCommand 兼容 HTTP 环境
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        try {
            document.execCommand('copy');
            btnCopy.textContent = '已复制';
            btnCopy.classList.add('copied');
            setTimeout(function() {
                btnCopy.textContent = '复制';
                btnCopy.classList.remove('copied');
            }, 2000);
        } catch (e) {
            btnCopy.textContent = '复制失败';
        }
        document.body.removeChild(textarea);
    });

    // 处理文件
    function handleFile(file) {
        // 仅接受 PNG 和 JPEG
        if (!file.type.match(/image\/(jpeg|png)/)) {
            showError('仅支持 PNG 和 JPEG 格式');
            return;
        }

        // 检查服务器配置的文件大小限制
        if (serverConfig && file.size > serverConfig.maxUploadBytes) {
            showError('文件过大（最大 ' + Math.round(serverConfig.maxUploadBytes / 1024 / 1024) + ' MiB）');
            return;
        }

        currentFile = file;
        hideError();
        hideResult();

        // 使用对象 URL 预览（比 Base64 更高效，不占用额外内存）
        if (currentObjectURL) {
            URL.revokeObjectURL(currentObjectURL);
        }
        currentObjectURL = URL.createObjectURL(file);
        previewImage.src = currentObjectURL;
        previewArea.style.display = 'block';
        uploadArea.style.display = 'none';
        btnDecode.disabled = false;
    }

    // 清除图片
    function clearImage() {
        currentFile = null;
        if (currentObjectURL) {
            URL.revokeObjectURL(currentObjectURL);
            currentObjectURL = null;
        }
        previewImage.src = '';
        previewArea.style.display = 'none';
        uploadArea.style.display = 'block';
        btnDecode.disabled = true;
        fileInput.value = '';
        hideResult();
        hideError();
    }

    // 解码图片
    function decodeImage(file) {
        btnDecode.disabled = true;
        btnDecode.classList.add('loading');
        hideError();
        hideResult();

        const formData = new FormData();
        formData.append('image', file);

        fetch('/api/decode', {
            method: 'POST',
            body: formData
        })
        .then(function(response) {
            return response.json();
        })
        .then(function(data) {
            btnDecode.classList.remove('loading');
            btnDecode.disabled = false;

            if (data.success) {
                showResult(data.text, data.format);
            } else {
                showError(data.error || '解码失败，未找到 EAN-13 条码');
            }
        })
        .catch(function(err) {
            btnDecode.classList.remove('loading');
            btnDecode.disabled = false;
            showError('请求失败：' + err.message);
        });
    }

    // 显示结果
    function showResult(text, format) {
        resultText.textContent = text;
        resultFormat.textContent = format || 'EAN_13';
        resultArea.style.display = 'block';
    }

    // 隐藏结果
    function hideResult() {
        resultArea.style.display = 'none';
    }

    // 显示错误
    function showError(message) {
        errorText.textContent = message;
        errorArea.style.display = 'block';
    }

    // 隐藏错误
    function hideError() {
        errorArea.style.display = 'none';
    }

    // 阻止页面拖拽默认行为（防止页面跳转）
    document.addEventListener('dragover', function(e) {
        e.preventDefault();
    });

    document.addEventListener('drop', function(e) {
        e.preventDefault();
    });
})();
