# K8s Resource Monitor - документация

## Обзор

K8s Resource Monitor - это легковесная утилита для мониторинга и анализа использования ресурсов (CPU и памяти) в кластере Kubernetes. Утилита работает как CLI-приложение, собирает данные о потреблении ресурсов подами и предоставляет инструменты для анализа, расчета стоимости и оптимизации.

Основные преимущества:
- Простота использования (требуется только сервисный аккаунт)
- Легковесность и высокая производительность
- Открытый исходный код и бесплатное использование
- Разработка, не зависящая от зарубежных решений
- 80% полезной функциональности без избыточной сложности

## Установка

1. Убедитесь, что у вас есть доступ к Kubernetes кластеру
2. Настройте сервисный аккаунт с необходимыми правами:
   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: resource-monitor
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: resource-monitor
   rules:
   - apiGroups: [""]
     resources: ["pods"]
     verbs: ["list", "get", "watch"]
   - apiGroups: ["metrics.k8s.io"]
     resources: ["pods"]
     verbs: ["get", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: resource-monitor
   subjects:
   - kind: ServiceAccount
     name: resource-monitor
     namespace: default
   roleRef:
     kind: ClusterRole
     name: resource-monitor
     apiGroup: rbac.authorization.k8s.io
   ```

## Команды

### Мониторинг ресурсов

Собирает данные о потреблении ресурсов подами с заданным интервалом.

```bash
k8s-monitor monitor [flags]
```

Флаги:
- `-i, --interval` - интервал сбора данных в секундах (по умолчанию: 10)
- `-o, --output` - путь к файлу для сохранения данных (по умолчанию: "/data/output.csv")
- `-n, --namespaces` - список namespace для фильтрации (через запятую)
- `-l, --labels` - фильтр по labels в формате key=value

Пример:
```bash
k8s-monitor monitor -i 30 -o metrics.csv -n default,production -l app=backend
```

### Отчет по использованию ресурсов

Генерирует отчет с ключевыми метриками потребления ресурсов.

```bash
k8s-monitor report [flags]
```

Флаги:
- `-f, --file` - файл с метриками (по умолчанию: "data.csv")
- `-l, --last` - период для анализа (1h, 24h, 7d) (по умолчанию: "24h")

Отчет включает:
- Общую статистику по CPU/памяти
- ТОП-5 подов по потреблению ресурсов
- Анализ по неймспейсам
- Выявление аномалий (когда под использовал >3x от среднего)

Пример:
```bash
k8s-monitor report -f metrics.csv -l 7d
```

### Очистка данных

Удаляет собранные данные мониторинга.

```bash
k8s-monitor reset [flags]
```

Флаги:
- `-f, --file` - файл с метриками (по умолчанию: "/data/output.csv")

Пример:
```bash
k8s-monitor reset -f metrics.csv
```

### Расчет стоимости

Анализирует стоимость потребления ресурсов кластера.

```bash
k8s-monitor cost [flags]
```

Флаги:
- `-f, --file` - файл с метриками (по умолчанию: "data.csv")
- `--cpu-price` - цена за 1 CPU-core/час ($) (по умолчанию: 0.02)
- `--mem-price` - цена за 1 GiB памяти/час ($) (по умолчанию: 0.01)

Отчет включает:
- Общую стоимость кластера
- Стоимость по неймспейсам
- ТОП-5 самых дорогих подов

Пример:
```bash
k8s-monitor cost -f metrics.csv --cpu-price 0.03 --mem-price 0.015
```

### Оптимизация ресурсов

Анализирует метрики и предлагает рекомендации по оптимизации.

```bash
k8s-monitor optimize [flags]
```

Флаги:
- `-f, --file` - файл с метриками (по умолчанию: "/data/output.csv")
- `-m, --margin` - запас прочности в % (по умолчанию: 20)

Функционал:
- Рекомендации по limits и requests для подов
- Анализ существующих limits/requests
- Расчет потенциальной экономии

Пример:
```bash
k8s-monitor optimize -f metrics.csv -m 15
```

## Формат данных

Данные сохраняются в CSV файл со следующими колонками:
- `timestamp` - время сбора метрик
- `namespace` - namespace пода
- `pod` - имя пода
- `CPU` - текущее использование CPU (в миллиядрах)
- `Memory` - текущее использование памяти (в Mi)
- `Status` - статус работы пода (OK, SKIP, ERROR)

## Примеры использования

1. Запуск мониторинга всех подов в namespace "production":
   ```bash
   k8s-monitor monitor -n production -o prod_metrics.csv
   ```

2. Генерация отчета за последние 7 дней:
   ```bash
   k8s-monitor report -f prod_metrics.csv -l 7d
   ```

3. Расчет стоимости с кастомными ценами:
   ```bash
   k8s-monitor cost --cpu-price 0.025 --mem-price 0.012
   ```

4. Оптимизация с запасом 25%:
   ```bash
   k8s-monitor optimize -m 25
   ```

## Лучшие практики

1. Для долгосрочного мониторинга используйте интервал 30-60 секунд
2. При анализе учитывайте периоды пиковой нагрузки
3. Регулярно проверяйте рекомендации по оптимизации
4. Используйте фильтры по namespace/labels для фокусировки на важных workload
5. Для production-кластеров сохраняйте исторические данные для анализа трендов

## Ограничения

1. Утилита не учитывает узлы кластера и их характеристики
2. Нет интеграции с системами алертинга
3. Анализ основан на текущих метриках без учета исторических трендов
4. Оптимизация предлагает базовые рекомендации без глубокого анализа