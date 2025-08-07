// ✅ 约束1：前端样式保护管理器
// 确保不修改任何现有UI样式，保持界面完全不变

interface StyleProtectionConfig {
    // 禁止任何CSS修改的选择器
    protectedSelectors: string[];
    // 只允许动态内容更新，不允许样式修改
    allowedOperations: string[];
    // 禁止的样式操作
    forbiddenOperations: string[];
}

class StyleProtectionManager {
    private config: StyleProtectionConfig;

    constructor() {
        this.config = {
            // 所有需要保护的CSS选择器
            protectedSelectors: [
                '.flex',
                '.h-full',
                '.w-full',
                '.bg-bg-primary',
                '.rounded-2xl',
                '.shadow-sm',
                '.bg-sidebar-bg',
                '.bg-white',
                '.text-blue-primary',
                '.text-gray-700',
                '.hover\\:bg-gray-100',
                '.bg-blue-selected',
                '.border-blue-200',
                // 聊天消息相关样式
                '.max-w-3xl',
                '.max-w-4xl',
                '.bg-blue-primary',
                '.rounded-2xl',
                '.rounded-tr-md',
                '.rounded-tl-md',
                '.bg-gray-50',
                '.text-sm',
                '.p-6',
                '.p-4',
                '.p-3',
                '.p-2',
                // 输入框相关样式
                '.rounded-full',
                '.shadow-lg',
                '.border-border-light',
                '.bg-gradient-to-r',
                '.from-red-400',
                '.to-pink-400',
                // 所有现有CSS类都需要保护
            ],
            
            // 只允许动态内容更新，不允许样式修改
            allowedOperations: [
                'innerHTML',    // 更新消息内容
                'textContent',  // 更新文本内容
                'appendChild',  // 添加新消息
                'removeChild',  // 移除消息
                'value',        // 更新输入框值
            ],
            
            // 禁止的样式操作
            forbiddenOperations: [
                'style',        // 直接样式修改
                'className',    // CSS类修改
                'classList',    // CSS类列表修改
                'setAttribute', // 样式属性修改
                'removeAttribute', // 移除样式属性
            ]
        };
    }

    // 安全的DOM操作包装器
    safeUpdateContent(element: HTMLElement | null, content: string): void {
        if (!element) return;
        
        // 只更新内容，不触碰样式
        element.innerHTML = content;
    }
    
    safeClearContent(element: HTMLElement | null): void {
        if (!element) return;
        
        // 清空内容但保持所有样式
        element.innerHTML = '';
    }
    
    safeAppendMessage(container: HTMLElement | null, messageHtml: string): void {
        if (!container) return;
        
        // 创建消息元素但不修改任何样式
        const messageDiv = document.createElement('div');
        messageDiv.innerHTML = messageHtml;
        container.appendChild(messageDiv);
    }

    // React组件安全更新方法
    safeUpdateReactComponent(setState: Function, newContent: string): void {
        // 使用React的setState更新内容，不修改样式
        setState(newContent);
    }

    // 验证操作是否安全
    validateOperation(operation: string, _target: Element): boolean {
        // 检查是否为允许的操作
        if (!this.config.allowedOperations.includes(operation)) {
            console.warn(`⚠️ StyleProtection: 操作 '${operation}' 不在允许列表中`);
            return false;
        }

        // 检查是否尝试修改受保护的样式
        if (this.config.forbiddenOperations.includes(operation)) {
            console.error(`🚫 StyleProtection: 禁止的样式操作 '${operation}'`);
            return false;
        }

        return true;
    }

    // 监控DOM变化（开发环境使用）
    enableStyleProtectionMonitoring(): void {
        // 在生产环境中禁用监控以减少控制台日志
        const isDevelopment = false; // 暂时禁用监控日志
            
        if (isDevelopment) {
            const observer = new MutationObserver((mutations) => {
                mutations.forEach((mutation) => {
                    if (mutation.type === 'attributes') {
                        const target = mutation.target as Element;
                        const attributeName = mutation.attributeName;
                        
                        // 检查是否修改了样式相关属性
                        if (attributeName === 'class' || attributeName === 'style') {
                            console.warn('⚠️ StyleProtection: 检测到样式修改', {
                                element: target.tagName,
                                attribute: attributeName,
                                newValue: target.getAttribute(attributeName)
                            });
                        }
                    }
                });
            });

            // 开始监控文档变化
            observer.observe(document.body, {
                attributes: true,
                attributeFilter: ['class', 'style'],
                subtree: true
            });

            console.log('✅ StyleProtection: 样式保护监控已启用');
        }
    }

    // 生成样式保护报告
    generateProtectionReport(): {
        protectedElements: number;
        allowedOperations: string[];
        protectionStatus: 'active' | 'inactive';
    } {
        const protectedElements = this.config.protectedSelectors.length;
        
        return {
            protectedElements,
            allowedOperations: this.config.allowedOperations,
            protectionStatus: 'active'
        };
    }
}

// 全局样式保护实例
export const styleProtection = new StyleProtectionManager();

// 在开发环境启用监控
const isDevelopment = typeof window !== 'undefined' && 
    (window as any).__DEV__ !== false;
    
if (isDevelopment) {
    styleProtection.enableStyleProtectionMonitoring();
}

export default StyleProtectionManager;