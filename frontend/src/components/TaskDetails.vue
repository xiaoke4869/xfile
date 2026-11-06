<script setup lang="ts">
import { vScrollbars } from '../scrollbars'
import { Check,Close,Download,FolderOpened,Loading,Warning } from '@element-plus/icons-vue'
import { size,statuses } from '../format'
import type { Task } from '../types'
defineProps<{task:Task}>()
const emit=defineEmits<{close:[];cancel:[];reports:[id:string];export:[id:string];recovery:[id:string];rescan:[]}>()
function taskPercent(task:Task){return task.total?Math.round(task.moved/task.total*100):0}
</script>

<template>
   <section class="task-dock" aria-live="polite"><div class="task-dock-title"><Loading v-if="task.status==='running'" class="spin"/><Check v-else-if="task.status==='succeeded'" class="green-text"/><Warning v-else class="amber-text"/><strong>{{statuses[task.status]||task.status}}</strong><span>{{task.phase}}</span><button v-if="task.status!=='running'" class="icon-button" aria-label="关闭任务结果" @click="emit('close')"><Close/></button><button v-else class="text-button" @click="emit('cancel')">取消任务</button></div><div class="progress-track"><div :style="{width:taskPercent(task)+'%'}"></div></div><div class="task-dock-summary"><span>{{task.moved}} / {{task.total}} 个文件 · {{size(task.doneBytes||0)}} / {{size(task.bytes||0)}}</span><span>{{taskPercent(task)}}%</span></div><p v-if="task.current" class="task-current" :title="task.current">{{task.current}}</p><p v-if="task.status==='running'&&task.copyBytes>0" class="task-current">当前文件已复制 {{size(task.copyBytes)}}</p><p v-scrollbars v-if="task.error" class="error-text">{{task.error}}</p><template v-if="task.status!=='running'&&task.directory"><div class="result-counts">目标原有 {{task.before}} · 执行后 {{task.after}} · 源剩余 {{task.remaining}} · 双份 {{task.retained}} · 待核对 {{task.unknown}}</div><div class="result-actions"><button class="button small" @click="emit('reports',task.id)"><FolderOpened/>打开日志</button><button class="button small" @click="emit('export',task.id)"><Download/>导出报告</button><button v-if="['cancelled','failed','recovery_required','needs_review'].includes(task.status)" class="button small" @click="emit('recovery',task.id)">核对并恢复</button></div></template><button v-if="task.status!=='running'&&task.status==='stale_plan'" class="button small" @click="emit('rescan')">重新扫描</button></section>
</template>
