document.addEventListener('DOMContentLoaded', function() {
    console.log('DOM đã tải xong. Khởi tạo ứng dụng...');
    
    // Ẩn khu vực kết quả khi mới tải trang
    if (document.getElementById('results-section')) {
        document.getElementById('results-section').classList.add('d-none');
        document.getElementById('results-section').style.display = 'none';
    }
    
    // Biến lưu trạng thái
    let uploadedFile = '';
    let isRunning = false;
    
    // Lấy giá trị từ server
    const runningValue = document.getElementById('running-state').getAttribute('data-value').toLowerCase();
    if (runningValue === "true") {
        isRunning = true;
    }
    
    let statusInterval;
    
    // Các elements
    const dropzone = document.getElementById('dropzone');
    const fileInput = document.getElementById('file-input');
    const uploadSuccess = document.getElementById('upload-success');
    const uploadedFilename = document.getElementById('uploaded-filename');
    const selectFile = document.getElementById('excel-file');
    const startBtn = document.getElementById('start-btn');
    const stopBtn = document.getElementById('stop-btn');
    const configForm = document.getElementById('config-form');
    const progressBar = document.getElementById('progress-bar');
    const progressText = document.getElementById('progress-text');
    const batchProgressBar = document.getElementById('batch-progress-bar');
    const batchText = document.getElementById('batch-text');
    const logContainer = document.getElementById('log-container');
    const changeFileBtn = document.getElementById('change-file-btn');
    const priceCard = document.getElementById('price-card');
    const resultsSection = document.getElementById('results-section');
    const downloadSuccessBtn = document.getElementById('download-success-btn');
    const downloadFailedBtn = document.getElementById('download-failed-btn');
    const successFileName = document.getElementById('success-file-name');
    const failedFileName = document.getElementById('failed-file-name');
    const uploadCard = document.querySelector('.card-upload .card-body');
    
    // Tạo timestamp hiện tại cho tên file
    const now = new Date();
    let sessionTimestamp = now.toISOString().replace(/[-:]/g, '').replace('T', '_').substring(0, 15);
    
    // Thiết lập URL download mặc định
    if (downloadSuccessBtn) {
        const filename = `b1_success_${sessionTimestamp}.xlsx`;
        downloadSuccessBtn.href = `/download/success/b1_success.xlsx?download=${filename}`;
        downloadSuccessBtn.setAttribute('download', filename);
    }
    
    if (downloadFailedBtn) {
        const filename = `b1_failed_${sessionTimestamp}.xlsx`;
        downloadFailedBtn.href = `/download/failed/b1_failed.xlsx?download=${filename}`;
        downloadFailedBtn.setAttribute('download', filename);
    }
    
    // Cập nhật tên file hiển thị với timestamp
    const formattedDate = formatTimestamp(sessionTimestamp);
    if (successFileName) {
        successFileName.textContent = `Danh sách tài khoản đúng (${formattedDate})`;
    }
    
    if (failedFileName) {
        failedFileName.textContent = `Danh sách tài khoản sai (${formattedDate})`;
    }
    
    // Các số liệu
    const totalAccountsEl = document.getElementById('total-accounts');
    const successAccountsEl = document.getElementById('success-accounts');
    const failedAccountsEl = document.getElementById('failed-accounts');
    const durationEl = document.getElementById('duration');
    
    // Cập nhật UI dựa trên trạng thái hiện tại
    updateUIBasedOnRunningState();
    
    // Khởi tạo: lấy danh sách file và kết quả
    fetchFileList();
    
    // Nếu đang chạy, khởi động interval để cập nhật trạng thái
    if (isRunning) {
        startStatusInterval();
        addLogMessage("Tiếp tục xử lý từ phiên trước...", "info");
    }
    
    // Lấy status khi load trang
    fetchStatus();
    
    // Lắng nghe thông báo hoàn thành từ server bằng polling
    startCompletionPolling();
    
    // Bắt đầu polling để kiểm tra trạng thái hoàn thành
    function startCompletionPolling() {
        console.log("Bắt đầu polling để kiểm tra trạng thái hoàn thành");
        // Kiểm tra mỗi 2 giây
        setInterval(checkCompletion, 2000);
    }
    
    // Kiểm tra nếu quá trình xử lý đã hoàn thành
    function checkCompletion() {
        if (!isRunning) {
            // Chỉ kiểm tra nếu trước đó hệ thống đang chạy
            return;
        }
        
        fetch('/status')
            .then(response => response.json())
            .then(data => {
                // Kiểm tra nếu đã chuyển từ đang chạy -> đã dừng
                if (!data.Running && data.ProcessedCount > 0) {
                    console.log("Phát hiện hoàn thành xử lý qua polling:", data);
                    // Dùng dữ liệu từ status để hiển thị kết quả
                    showProcessingResults(data);
                    isRunning = false;
                    updateUIBasedOnRunningState();
                }
            })
            .catch(error => {
                console.error("Lỗi khi kiểm tra trạng thái hoàn thành:", error);
            });
    }
    
    // Định dạng timestamp thành DD/MM/YYYY HH:MM:SS (giờ Việt Nam)
    function formatTimestamp(timestamp) {
        if (timestamp && timestamp.length >= 15) {
            try {
                // Tách timestamp từ định dạng YYYYMMDD_HHMMSS
                const year = timestamp.substring(0, 4);
                const month = timestamp.substring(4, 6);
                const day = timestamp.substring(6, 8);
                const hour = timestamp.substring(9, 11);
                const minute = timestamp.substring(11, 13);
                const second = timestamp.substring(13, 15);
                
                // Tạo định dạng ngày giờ Việt Nam
                return `${day}/${month}/${year} ${hour}:${minute}:${second}`;
            } catch (e) {
                console.error("Lỗi định dạng timestamp:", e);
                return timestamp;
            }
        }
        return timestamp;
    }
    
    // Xử lý kéo thả file
    dropzone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzone.classList.add('dragover');
    });
    
    dropzone.addEventListener('dragleave', () => {
        dropzone.classList.remove('dragover');
    });
    
    dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('dragover');
        
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            handleFileUpload(files[0]);
        }
    });
    
    dropzone.addEventListener('click', () => {
        fileInput.click();
    });
    
    fileInput.addEventListener('change', (e) => {
        if (e.target.files.length > 0) {
            handleFileUpload(e.target.files[0]);
        }
    });
    
    // Nút để đổi file khác
    if (changeFileBtn) {
        changeFileBtn.addEventListener('click', () => {
            // Hiện lại khu vực upload
            dropzone.style.display = 'block';
            uploadSuccess.classList.add('d-none');
        });
    }
    
    // Xử lý form submit
    configForm.addEventListener('submit', (e) => {
        e.preventDefault();
        if (isRunning) {
            stopProcessing();
        } else {
            startProcessing();
        }
    });
    
    // Xử lý nút dừng
    stopBtn.addEventListener('click', () => {
        stopProcessing();
    });
    
    // Hàm xử lý upload file
    function handleFileUpload(file) {
        // Kiểm tra phần mở rộng file
        if (!file.name.endsWith('.xlsx')) {
            addLogMessage('Chỉ hỗ trợ file Excel (.xlsx)', 'error');
            return;
        }
        
        const formData = new FormData();
        formData.append('excel_file', file);
        
        addLogMessage(`Đang tải lên file ${file.name}...`, 'info');
        
        fetch('/upload', {
            method: 'POST',
            body: formData
        })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                addLogMessage(`Lỗi: ${data.error}`, 'error');
            } else {
                uploadedFile = data.file;
                uploadedFilename.textContent = data.file;
                uploadSuccess.classList.remove('d-none');
                
                // Ẩn khu vực kéo thả file sau khi tải lên thành công
                dropzone.style.display = 'none';
                
                // Hiển thị báo giá ngay sau khi tải file thành công
                analyzeExcelFile(data.file);
                
                addLogMessage(`Đã tải lên file ${data.file} thành công`, 'success');
                
                // Cập nhật danh sách file
                fetchFileList();
            }
        })
        .catch(error => {
            console.error('Error:', error);
            addLogMessage('Lỗi khi tải lên file', 'error');
        });
    }
    
    // Phân tích file Excel để hiển thị báo giá
    function analyzeExcelFile(fileName) {
        fetch(`/analyze?file=${encodeURIComponent(fileName)}`)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('Error:', data.error);
                return;
            }
            
            if (data.total_accounts > 0) {
                const accountCount = document.getElementById('account-count');
                const totalPrice = document.getElementById('total-price');
                
                // Hiển thị số lượng tài khoản và giá tiền (200 VNĐ/tài khoản)
                accountCount.textContent = data.total_accounts;
                totalPrice.textContent = (data.total_accounts * 200).toLocaleString('vi-VN') + ' VNĐ';
                
                // Hiển thị thẻ báo giá
                if (priceCard) {
                    priceCard.style.display = 'block';
                }
            } else {
                // Nếu không lấy được số tài khoản từ server, sử dụng giá trị mặc định
                const accountCount = document.getElementById('account-count');
                const totalPrice = document.getElementById('total-price');
                
                // Hiển thị thông báo mặc định
                accountCount.textContent = "0";
                totalPrice.textContent = "0 VNĐ";
                
                // Vẫn hiển thị thẻ báo giá
                if (priceCard) {
                    priceCard.style.display = 'block';
                }
            }
        })
        .catch(error => {
            console.error('Error:', error);
            // Nếu có lỗi khi phân tích, vẫn hiển thị thẻ báo giá với giá trị mặc định
            const accountCount = document.getElementById('account-count');
            const totalPrice = document.getElementById('total-price');
            
            accountCount.textContent = "0";
            totalPrice.textContent = "0 VNĐ";
            
            if (priceCard) {
                priceCard.style.display = 'block';
            }
        });
    }
    
    // Lấy danh sách file
    function fetchFileList() {
        fetch('/files')
        .then(response => response.json())
        .then(data => {
            // Xóa các option cũ
            while(selectFile.options.length > 1) {
                selectFile.remove(1);
            }
            
            // Thêm các file mới
            data.forEach(file => {
                const option = document.createElement('option');
                option.value = file;
                option.textContent = file;
                
                // Chọn file vừa upload
                if (file === uploadedFile) {
                    option.selected = true;
                }
                
                selectFile.appendChild(option);
            });
        })
        .catch(error => {
            console.error('Error:', error);
            addLogMessage('Không thể lấy danh sách file', 'error');
        });
    }
    
    // Bắt đầu xử lý
    function startProcessing() {
        const formData = new FormData(configForm);
        
        if (!formData.get('excel_file')) {
            addLogMessage('Vui lòng chọn file Excel trước khi bắt đầu', 'warning');
            return;
        }
        
        addLogMessage('Đang gửi yêu cầu xử lý...', 'info');
        
        // Ẩn khu vực kết quả khi bắt đầu một phiên xử lý mới
        if (document.getElementById('results-section')) {
            document.getElementById('results-section').classList.add('d-none');
        }
        
        fetch('/start', {
            method: 'POST',
            body: formData
        })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                addLogMessage(`Lỗi: ${data.error}`, 'error');
            } else {
                isRunning = true;
                updateUIBasedOnRunningState();
                addLogMessage('Đã bắt đầu xử lý tài khoản', 'success');
                startStatusInterval();
            }
        })
        .catch(error => {
            console.error('Error:', error);
            addLogMessage('Lỗi khi bắt đầu xử lý', 'error');
        });
    }
    
    // Dừng xử lý
    function stopProcessing() {
        addLogMessage('Đang dừng xử lý...', 'warning');
        
        fetch('/stop', {
            method: 'POST'
        })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                addLogMessage(`Lỗi: ${data.error}`, 'error');
            } else {
                isRunning = false;
                updateUIBasedOnRunningState();
                addLogMessage('Đã dừng xử lý tài khoản', 'info');
                clearInterval(statusInterval);
                
                // Đợi 1 giây để lấy kết quả sau khi dừng
                setTimeout(() => {
                    fetch('/status')
                        .then(response => response.json())
                        .then(statusData => {
                            console.log("Dữ liệu sau khi dừng:", statusData);
                            // Kiểm tra và hiển thị kết quả nếu có dữ liệu
                            if (statusData.ProcessedCount > 0) {
                                showProcessingResults(statusData);
                            }
                        })
                        .catch(error => {
                            console.error("Lỗi khi lấy trạng thái sau khi dừng:", error);
                        });
                }, 1000);
            }
        })
        .catch(error => {
            console.error('Error:', error);
            addLogMessage('Lỗi khi dừng xử lý', 'error');
        });
    }
    
    // Khởi động interval để cập nhật trạng thái
    function startStatusInterval() {
        if (statusInterval) {
            clearInterval(statusInterval);
        }
        
        statusInterval = setInterval(fetchStatus, 1000);
    }
    
    // Lấy trạng thái hiện tại
    function fetchStatus() {
        fetch('/status')
        .then(response => response.json())
        .then(data => {
            // Cập nhật biến trạng thái
            const wasRunning = isRunning;
            isRunning = data.Running;
            
            // Cập nhật các số liệu
            totalAccountsEl.textContent = data.TotalAccounts;
            successAccountsEl.textContent = data.SuccessCount;
            failedAccountsEl.textContent = data.FailedCount;
            if (data.Duration) {
                durationEl.textContent = data.Duration;
            }
            
            // Hiển thị giá tiền nếu có tài khoản
            if (data.TotalAccounts > 0) {
                const accountCount = document.getElementById('account-count');
                const totalPrice = document.getElementById('total-price');
                
                accountCount.textContent = data.TotalAccounts;
                totalPrice.textContent = (data.TotalAccounts * 200).toLocaleString('vi-VN') + ' VNĐ';
                
                // Hiển thị thẻ báo giá
                if (priceCard) {
                    priceCard.style.display = 'block';
                }
                
                // Cập nhật tiến độ
                const progress = (data.ProcessedCount / data.TotalAccounts) * 100;
                progressBar.style.width = `${progress}%`;
                progressText.textContent = `${data.ProcessedCount}/${data.TotalAccounts}`;
            }
            
            // Cập nhật batch progress
            if (data.TotalBatches > 0) {
                const batchProgress = (data.CurrentBatch / data.TotalBatches) * 100;
                batchProgressBar.style.width = `${batchProgress}%`;
                batchText.textContent = `${data.CurrentBatch}/${data.TotalBatches}`;
            }
            
            // Kiểm tra xem có phải đã hoàn thành xử lý hay không
            // 1. Nếu trước đó đang chạy (wasRunning) và hiện tại không chạy nữa (!isRunning)
            // 2. Và đã có dữ liệu được xử lý (TotalAccounts > 0 và ProcessedCount > 0)
            // 3. Và đã xử lý xong toàn bộ (ProcessedCount == TotalAccounts)
            if (wasRunning && !isRunning && 
                data.TotalAccounts > 0 && 
                data.ProcessedCount > 0 && 
                data.ProcessedCount >= data.TotalAccounts) {
                
                console.log("Đã hoàn thành xử lý, hiển thị kết quả");
                // Hiển thị kết quả
                showProcessingResults(data);
            }
            
            // Cập nhật UI
            updateUIBasedOnRunningState();
        })
        .catch(error => {
            console.error('Error:', error);
        });
    }
    
    // Hiển thị kết quả xử lý
    function showProcessingResults(data) {
        console.log("Hiển thị kết quả xử lý:", data);
        
        // Dừng interval status nếu đang chạy
        if (statusInterval) {
            clearInterval(statusInterval);
        }
        
        // Thông báo hoàn thành
        addLogMessage(`Hoàn thành xử lý ${data.ProcessedCount} tài khoản (${data.SuccessCount} thành công, ${data.FailedCount} thất bại) trong ${data.Duration}`, 'success');
        
        // Lấy timestamp từ server nếu có
        if (data.SessionTimestamp) {
            sessionTimestamp = data.SessionTimestamp;
        }
        
        // Hiển thị kết quả nếu có dữ liệu
        if (data.SuccessCount > 0 || data.FailedCount > 0) {
            processingCompleted(data.SuccessCount, data.FailedCount, sessionTimestamp);
        }
    }
    
    // Xử lý khi hoàn thành xử lý
    function processingCompleted(successCount, failedCount, timestamp) {
        console.log("Hoàn thành xử lý - hiển thị kết quả");
        
        // Thông báo hoàn thành xử lý
        addLogMessage("ĐÃ XỬ LÝ XONG TÀI KHOẢN! Vui lòng tải file kết quả bên dưới.", "success");
        
        // Hiển thị khu vực kết quả
        if (resultsSection) {
            resultsSection.classList.remove('d-none');
            resultsSection.style.display = 'block';
            
            // Cuộn đến khu vực kết quả
            setTimeout(() => {
                resultsSection.scrollIntoView({ behavior: 'smooth' });
            }, 300);
        }
        
        // Hiển thị thông báo popup
        alert("ĐÃ HOÀN THÀNH XỬ LÝ TÀI KHOẢN!\nVui lòng tải xuống file kết quả bên dưới.");
        
        // Cập nhật thông tin hiển thị kết quả
        updateResultDisplay(timestamp, successCount, failedCount);
    }
    
    // Cập nhật UI dựa trên trạng thái
    function updateUIBasedOnRunningState() {
        if (isRunning) {
            startBtn.classList.add('d-none');
            stopBtn.classList.remove('d-none');
            dropzone.classList.add('disabled');
            fileInput.disabled = true;
            selectFile.disabled = true;
            document.getElementById('max-workers').disabled = true;
            document.getElementById('start-row').disabled = true;
            document.getElementById('save-results').disabled = true;
        } else {
            startBtn.classList.remove('d-none');
            stopBtn.classList.add('d-none');
            dropzone.classList.remove('disabled');
            fileInput.disabled = false;
            selectFile.disabled = false;
            document.getElementById('max-workers').disabled = false;
            document.getElementById('start-row').disabled = false;
            document.getElementById('save-results').disabled = false;
        }
    }
    
    // Thêm thông báo vào log
    function addLogMessage(message, type = 'info') {
        const logMessage = document.createElement('div');
        logMessage.classList.add('log-message', `log-${type}`);
        
        const timestamp = new Date().toLocaleTimeString();
        const icon = getIconForLogType(type);
        
        logMessage.innerHTML = `<i class="${icon} me-2"></i>${message}`;
        
        logContainer.appendChild(logMessage);
        logContainer.scrollTop = logContainer.scrollHeight;
        
        // Giới hạn số lượng log
        while (logContainer.children.length > 100) {
            logContainer.removeChild(logContainer.firstChild);
        }
    }
    
    // Lấy icon cho loại log
    function getIconForLogType(type) {
        switch(type) {
            case 'success': return 'bi bi-check-circle';
            case 'error': return 'bi bi-x-circle';
            case 'warning': return 'bi bi-exclamation-triangle';
            default: return 'bi bi-info-circle';
        }
    }
    
    // Cập nhật hiển thị kết quả
    function updateResultDisplay(timestamp, successCount, failedCount) {
        // Hiển thị thông tin chi tiết về kết quả
        addLogMessage(`Kết quả: ${successCount} tài khoản thành công, ${failedCount} tài khoản thất bại.`, "info");
        
        // Định dạng timestamp
        let formattedDate = formatTimestamp(timestamp);
        
        // Cập nhật tóm tắt kết quả
        const resultSummary = document.getElementById('result-summary');
        if (resultSummary) {
            resultSummary.textContent = `Kết quả kiểm tra tài khoản: ${successCount} thành công, ${failedCount} thất bại`;
            resultSummary.style.fontWeight = 'bold';
            resultSummary.style.color = '#0066cc';
        }
        
        // Cập nhật tên file hiển thị với timestamp
        if (successFileName) {
            successFileName.textContent = `Danh sách tài khoản đúng (${formattedDate})`;
            successFileName.style.fontWeight = 'bold';
        }
        
        if (failedFileName) {
            failedFileName.textContent = `Danh sách tài khoản sai (${formattedDate})`;
            failedFileName.style.fontWeight = 'bold';
        }
        
        // Cập nhật link download với timestamp trong tên file
        if (downloadSuccessBtn) {
            const filename = `b1_success_${timestamp}.xlsx`;
            downloadSuccessBtn.href = `/download/success/b1_success.xlsx?download=${filename}`;
            downloadSuccessBtn.setAttribute('download', filename);
            
            // Thêm hiệu ứng để thu hút sự chú ý nếu có tài khoản thành công
            if (successCount > 0) {
                downloadSuccessBtn.classList.add('btn-pulse');
            }
        }
        
        if (downloadFailedBtn) {
            const filename = `b1_failed_${timestamp}.xlsx`;
            downloadFailedBtn.href = `/download/failed/b1_failed.xlsx?download=${filename}`;
            downloadFailedBtn.setAttribute('download', filename);
            
            // Thêm hiệu ứng để thu hút sự chú ý nếu có tài khoản thất bại
            if (failedCount > 0) {
                downloadFailedBtn.classList.add('btn-pulse');
            }
        }
    }
}); 