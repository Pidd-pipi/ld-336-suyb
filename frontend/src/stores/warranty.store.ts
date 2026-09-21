import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { WarrantyAlertItem } from '../models';
import { deviceWarrantyAlertsApi, WarrantyAlertQuery } from '../api/device.api';

// 保修到期预警 store：仅保存后端计算好的分类与剩余天数，刷新时按上次筛选条件回读。
@Injectable({ providedIn: 'root' })
export class WarrantyStore {
  readonly list = signal<WarrantyAlertItem[]>([]);
  readonly loading = signal(false);
  private lastQuery: WarrantyAlertQuery = {};

  constructor(private http: HttpClient) {}

  load(q: WarrantyAlertQuery): void {
    this.lastQuery = q;
    this.loading.set(true);
    deviceWarrantyAlertsApi(this.http, q).subscribe({
      next: (list) => {
        this.list.set(list);
        this.loading.set(false);
      },
      error: () => {
        this.list.set([]);
        this.loading.set(false);
      },
    });
  }

  // 刷新：按上次筛选条件重新回读。
  reload(): void {
    this.load(this.lastQuery);
  }
}
