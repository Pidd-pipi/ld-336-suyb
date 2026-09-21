import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Device, PageResult, WarrantyAlert } from '../models';
import { deviceDisableApi, deviceEnableApi, deviceListApi, deviceWarrantyAlertsApi, DeviceQuery, WarrantyAlertQuery } from '../api/device.api';

@Injectable({ providedIn: 'root' })
export class DeviceStore {
  readonly devices = signal<Device[]>([]);
  readonly total = signal(0);
  readonly loading = signal(false);
  // 保修到期预警清单与过滤条件（后端统一计算分类，前端只展示）。
  readonly warrantyAlerts = signal<WarrantyAlert[]>([]);
  readonly alertsLoading = signal(false);
  private lastQuery: DeviceQuery | null = null;
  private lastAlertQuery: WarrantyAlertQuery | null = null;

  constructor(private http: HttpClient) {}

  load(q: DeviceQuery): void {
    this.lastQuery = q;
    this.loading.set(true);
    deviceListApi(this.http, q).subscribe({
      next: (res: PageResult<Device>) => {
        this.devices.set(res.list);
        this.total.set(res.total);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  // loadWarrantyAlerts 拉取保修到期预警清单并回读刷新（科室/类别/预警类型过滤）。
  loadWarrantyAlerts(q: WarrantyAlertQuery): void {
    this.lastAlertQuery = q;
    this.alertsLoading.set(true);
    deviceWarrantyAlertsApi(this.http, q).subscribe({
      next: (list) => {
        this.warrantyAlerts.set(list);
        this.alertsLoading.set(false);
      },
      error: () => {
        this.warrantyAlerts.set([]);
        this.alertsLoading.set(false);
      },
    });
  }

  disable(id: number): void {
    deviceDisableApi(this.http, id).subscribe(() => this.reload());
  }

  enable(id: number): void {
    deviceEnableApi(this.http, id).subscribe(() => this.reload());
  }

  // reload 刷新当前视图：台账列表与保修预警清单均回读（禁用/启用后预警清单同步更新）。
  reload(): void {
    if (this.lastQuery) {
      this.load(this.lastQuery);
    }
    if (this.lastAlertQuery) {
      this.loadWarrantyAlerts(this.lastAlertQuery);
    }
  }
}
