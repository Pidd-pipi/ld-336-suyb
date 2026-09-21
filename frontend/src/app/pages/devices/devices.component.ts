import { Component, inject, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatDialog, MatDialogModule } from '@angular/material/dialog';
import { MatCardModule } from '@angular/material/card';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatPaginatorModule } from '@angular/material/paginator';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { PageHeaderComponent } from '../../components/page-header/page-header.component';
import { StatusBadgeComponent } from '../../components/status-badge/status-badge.component';
import { EmptyStateComponent } from '../../components/empty-state/empty-state.component';
import { ConfirmDialogComponent, ConfirmDialogData } from '../../components/confirm-dialog/confirm-dialog.component';
import { DeviceFormDialogComponent, DeviceFormData } from './device-form-dialog.component';
import { DeviceStore } from '../../../stores/device.store';
import { Device, WarrantyAlert } from '../../../models';
import { deviceCreateApi, deviceUpdateApi } from '../../../api/device.api';
import { DEVICE_STATUS, DEVICE_STATUS_TEXT, DEPARTMENTS, DEVICE_CATEGORIES, WARRANTY_ALERT, WARRANTY_ALERT_TEXT } from '../../../constants/enums';
import { formatDate, moneyLabel } from '../../../utils/format';
import { parseHttpError, useHttp } from '../../../utils/request';
import { Subject, takeUntil } from 'rxjs';

@Component({
  selector: 'app-devices',
  standalone: true,
  imports: [
    MatCardModule,
    CommonModule, ReactiveFormsModule, MatTableModule, MatButtonModule, MatIconModule, MatFormFieldModule,
    MatInputModule, MatSelectModule, MatDialogModule, MatProgressSpinnerModule, MatPaginatorModule,
    MatTooltipModule, MatSnackBarModule, PageHeaderComponent, StatusBadgeComponent, EmptyStateComponent,
  ],
  template: `
    <app-page-header title="设备台账" subtitle="全院医疗器械电子台账，支持多维度检索与保修到期预警"></app-page-header>

    <div class="view-switch">
      <button mat-flat-button [color]="mode === 'ledger' ? 'primary' : undefined" (click)="switchMode('ledger')">
        <mat-icon>devices</mat-icon> 设备台账
      </button>
      <button mat-flat-button [color]="mode === 'warranty' ? 'primary' : undefined" (click)="switchMode('warranty')">
        <mat-icon>warning</mat-icon> 保修预警
        <span class="alert-count" *ngIf="store.warrantyAlerts().length > 0">{{ store.warrantyAlerts().length }}</span>
      </button>
    </div>

    <!-- 设备台账视图（保持原有检索与状态管理） -->
    <ng-container *ngIf="mode === 'ledger'">
      <form class="filter-bar" [formGroup]="form">
        <mat-form-field appearance="outline">
          <mat-label>关键词</mat-label>
          <input matInput formControlName="keyword" (keyup.enter)="search()" placeholder="名称/编号/序列号">
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>科室</mat-label>
          <mat-select formControlName="department">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let d of departments" [value]="d">{{ d }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>设备类型</mat-label>
          <mat-select formControlName="category">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let c of categories" [value]="c">{{ c }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>状态</mat-label>
          <mat-select formControlName="status">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let s of statusOptions" [value]="s.value">{{ s.label }}</mat-option>
          </mat-select>
        </mat-form-field>
        <div class="spacer"></div>
        <button mat-flat-button color="primary" (click)="search()"><mat-icon>search</mat-icon> 查询</button>
        <button mat-flat-button color="accent" (click)="openCreate()"><mat-icon>add</mat-icon> 登记设备</button>
      </form>
      <mat-card>
        <div class="table-wrap">
          <table mat-table [dataSource]="store.devices()" class="full-table">
            <ng-container matColumnDef="asset_code">
              <th mat-header-cell *matHeaderCellDef>资产编号</th>
              <td mat-cell *matCellDef="let d">{{ d.asset_code }}</td>
            </ng-container>
            <ng-container matColumnDef="name">
              <th mat-header-cell *matHeaderCellDef>设备名称</th>
              <td mat-cell *matCellDef="let d">{{ d.name }}</td>
            </ng-container>
            <ng-container matColumnDef="category">
              <th mat-header-cell *matHeaderCellDef>类型</th>
              <td mat-cell *matCellDef="let d">{{ d.category || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="department">
              <th mat-header-cell *matHeaderCellDef>科室</th>
              <td mat-cell *matCellDef="let d">{{ d.department || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="manufacturer">
              <th mat-header-cell *matHeaderCellDef>厂家</th>
              <td mat-cell *matCellDef="let d">{{ d.manufacturer || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="purchase_amount">
              <th mat-header-cell *matHeaderCellDef>购置金额</th>
              <td mat-cell *matCellDef="let d">{{ moneyLabel(d.purchase_amount) }}</td>
            </ng-container>
            <ng-container matColumnDef="warranty_expiry">
              <th mat-header-cell *matHeaderCellDef>保修到期</th>
              <td mat-cell *matCellDef="let d">
                {{ formatDate(d.warranty_expiry) }}
                <mat-icon *ngIf="d.warranty_expired" class="warn" matTooltip="保修已过期">warning</mat-icon>
              </td>
            </ng-container>
            <ng-container matColumnDef="status">
              <th mat-header-cell *matHeaderCellDef>状态</th>
              <td mat-cell *matCellDef="let d"><app-status-badge [status]="d.status" [labelMap]="statusText"></app-status-badge></td>
            </ng-container>
            <ng-container matColumnDef="actions">
              <th mat-header-cell *matHeaderCellDef>操作</th>
              <td mat-cell *matCellDef="let d">
                <button mat-icon-button matTooltip="编辑" (click)="openEdit(d)"><mat-icon>edit</mat-icon></button>
                <button mat-icon-button matTooltip="禁用" *ngIf="canDisable(d)" (click)="disable(d)"><mat-icon>block</mat-icon></button>
                <button mat-icon-button matTooltip="启用" *ngIf="d.status === 'disabled'" (click)="enable(d)"><mat-icon>check_circle</mat-icon></button>
              </td>
            </ng-container>
            <tr mat-header-row *matHeaderRowDef="columns"></tr>
            <tr mat-row *matRowDef="let row; columns: columns;"></tr>
          </table>
          <app-empty-state *ngIf="!store.loading() && store.devices().length === 0" message="暂无设备数据，请点击登记设备"></app-empty-state>
          <div class="loading" *ngIf="store.loading()"><mat-spinner diameter="30"></mat-spinner></div>
        </div>
        <mat-paginator
          [length]="store.total()" [pageSize]="pageSize" [pageSizeOptions]="[5, 10, 20]"
          (page)="onPage($event)">
        </mat-paginator>
      </mat-card>
    </ng-container>

    <!-- 保修到期预警视图（科室/类别/预警类型筛选，展示责任人与剩余天数） -->
    <ng-container *ngIf="mode === 'warranty'">
      <form class="filter-bar" [formGroup]="alertForm">
        <mat-form-field appearance="outline">
          <mat-label>科室</mat-label>
          <mat-select formControlName="department">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let d of departments" [value]="d">{{ d }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>设备类型</mat-label>
          <mat-select formControlName="category">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let c of categories" [value]="c">{{ c }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>预警类型</mat-label>
          <mat-select formControlName="alert_type">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let a of alertOptions" [value]="a.value">{{ a.label }}</mat-option>
          </mat-select>
        </mat-form-field>
        <div class="spacer"></div>
        <button mat-flat-button color="primary" (click)="searchAlerts()"><mat-icon>search</mat-icon> 查询</button>
        <button mat-stroked-button color="primary" (click)="refreshAlerts()"><mat-icon>refresh</mat-icon> 刷新</button>
      </form>
      <mat-card>
        <div class="alert-summary">
          共 {{ store.warrantyAlerts().length }} 台设备需要关注：
          <span class="tag-expired">已过保 {{ countByType(WARRANTY_ALERT.EXPIRED) }} 台</span>
          <span class="tag-due">三十天内到期 {{ countByType(WARRANTY_ALERT.DUE) }} 台</span>
          <span class="hint">（已报废、已禁用设备不进入清单）</span>
        </div>
        <div class="table-wrap">
          <table mat-table [dataSource]="store.warrantyAlerts()" class="full-table">
            <ng-container matColumnDef="asset_code">
              <th mat-header-cell *matHeaderCellDef>资产编号</th>
              <td mat-cell *matCellDef="let a">{{ a.asset_code }}</td>
            </ng-container>
            <ng-container matColumnDef="name">
              <th mat-header-cell *matHeaderCellDef>设备名称</th>
              <td mat-cell *matCellDef="let a">{{ a.name }}</td>
            </ng-container>
            <ng-container matColumnDef="category">
              <th mat-header-cell *matHeaderCellDef>类型</th>
              <td mat-cell *matCellDef="let a">{{ a.category || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="department">
              <th mat-header-cell *matHeaderCellDef>科室</th>
              <td mat-cell *matCellDef="let a">{{ a.department || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="responsible_person">
              <th mat-header-cell *matHeaderCellDef>责任人</th>
              <td mat-cell *matCellDef="let a">{{ a.responsible_person || '-' }}</td>
            </ng-container>
            <ng-container matColumnDef="warranty_expiry">
              <th mat-header-cell *matHeaderCellDef>保修到期日</th>
              <td mat-cell *matCellDef="let a">{{ formatDate(a.warranty_expiry) }}</td>
            </ng-container>
            <ng-container matColumnDef="warranty_days">
              <th mat-header-cell *matHeaderCellDef>剩余天数</th>
              <td mat-cell *matCellDef="let a">
                <span [class.days-expired]="a.alert_type === 'expired'" [class.days-due]="a.alert_type === 'due'">{{ daysLabel(a) }}</span>
              </td>
            </ng-container>
            <ng-container matColumnDef="alert_type">
              <th mat-header-cell *matHeaderCellDef>预警类型</th>
              <td mat-cell *matCellDef="let a"><app-status-badge [status]="a.alert_type" [labelMap]="warrantyText"></app-status-badge></td>
            </ng-container>
            <tr mat-header-row *matHeaderRowDef="alertColumns"></tr>
            <tr mat-row *matRowDef="let row; columns: alertColumns;"></tr>
          </table>
          <app-empty-state *ngIf="!store.alertsLoading() && store.warrantyAlerts().length === 0" message="暂无保修到期预警设备"></app-empty-state>
          <div class="loading" *ngIf="store.alertsLoading()"><mat-spinner diameter="30"></mat-spinner></div>
        </div>
      </mat-card>
    </ng-container>
  `,
  styles: [`
    .full-table { width: 100%; }
    .warn { color: #f44336; font-size: 16px; vertical-align: middle; }
    .loading { display: flex; justify-content: center; padding: 24px; }
    .view-switch { display: flex; gap: 8px; margin: 16px 0; }
    .alert-count { display: inline-block; min-width: 18px; padding: 0 6px; margin-left: 6px; border-radius: 9px;
      background: #f44336; color: #fff; font-size: 12px; line-height: 18px; text-align: center; }
    .alert-summary { padding: 8px 4px 12px; font-size: 13px; color: #555; }
    .alert-summary .tag-expired { margin-left: 8px; color: #c62828; font-weight: 600; }
    .alert-summary .tag-due { margin-left: 12px; color: #ef6c00; font-weight: 600; }
    .alert-summary .hint { margin-left: 12px; color: #999; }
    .days-expired { color: #c62828; font-weight: 600; }
    .days-due { color: #ef6c00; font-weight: 600; }
  `],
})
export class DevicesComponent implements OnInit, OnDestroy {
  private fb = inject(FormBuilder);
  private http = useHttp();
  private dialog = inject(MatDialog);
  private snackBar = inject(MatSnackBar);
  private destroy$ = new Subject<void>();
  store = inject(DeviceStore);

  mode: 'ledger' | 'warranty' = 'ledger';
  statusText = DEVICE_STATUS_TEXT;
  warrantyText = WARRANTY_ALERT_TEXT;
  departments = DEPARTMENTS;
  categories = DEVICE_CATEGORIES;
  statusOptions = Object.entries(DEVICE_STATUS_TEXT).map(([value, label]) => ({ value, label }));
  alertOptions = Object.entries(WARRANTY_ALERT_TEXT).map(([value, label]) => ({ value, label }));
  columns = ['asset_code', 'name', 'category', 'department', 'manufacturer', 'purchase_amount', 'warranty_expiry', 'status', 'actions'];
  alertColumns = ['asset_code', 'name', 'category', 'department', 'responsible_person', 'warranty_expiry', 'warranty_days', 'alert_type'];
  page = 1;
  pageSize = 10;

  form = this.fb.nonNullable.group({
    keyword: [''],
    department: [''],
    category: [''],
    status: [''],
  });

  // 预警筛选表单：科室 / 类别 / 预警类型。
  alertForm = this.fb.nonNullable.group({
    department: [''],
    category: [''],
    alert_type: [''],
  });

  ngOnInit(): void {
    this.load();
    // 进入设备页即回读一次预警清单，用于角标计数（分类与剩余天数以后端返回为准）。
    this.loadAlerts();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  switchMode(m: 'ledger' | 'warranty'): void {
    this.mode = m;
    if (m === 'warranty') {
      // 切到预警视图时回读最新结果。
      this.loadAlerts();
    } else {
      // 切回台账时重置预警筛选并回读全量，保证角标计数始终是全部待关注数。
      this.alertForm.reset({ department: '', category: '', alert_type: '' });
      this.loadAlerts();
    }
  }

  load(): void {
    this.store.load({
      page: this.page,
      page_size: this.pageSize,
      department: this.form.value.department || undefined,
      category: this.form.value.category || undefined,
      status: this.form.value.status || undefined,
      keyword: this.form.value.keyword || undefined,
    });
  }

  // loadAlerts 按当前筛选条件向后端查询保修预警（后端统一分类，前端只展示）。
  loadAlerts(): void {
    this.store.loadWarrantyAlerts({
      department: this.alertForm.value.department || undefined,
      category: this.alertForm.value.category || undefined,
      alert_type: this.alertForm.value.alert_type || undefined,
    });
  }

  search(): void {
    this.page = 1;
    this.load();
  }

  searchAlerts(): void {
    this.loadAlerts();
  }

  // refreshAlerts 手动刷新后从后端回读。
  refreshAlerts(): void {
    this.loadAlerts();
  }

  countByType(type: string): number {
    return this.store.warrantyAlerts().filter((a) => a.alert_type === type).length;
  }

  // daysLabel 仅基于后端计算好的 warranty_days 展示，不在前端重新分类。
  daysLabel(a: WarrantyAlert): string {
    if (a.alert_type === WARRANTY_ALERT.EXPIRED) {
      return `已过保 ${Math.abs(a.warranty_days)} 天`;
    }
    return a.warranty_days === 0 ? '今天到期' : `剩余 ${a.warranty_days} 天`;
  }

  onPage(e: { pageIndex: number; pageSize: number }): void {
    this.page = e.pageIndex + 1;
    this.pageSize = e.pageSize;
    this.load();
  }

  canDisable(d: Device): boolean {
    return d.status !== DEVICE_STATUS.DISABLED && d.status !== DEVICE_STATUS.SCRAPPED;
  }

  openCreate(): void {
    const ref = this.dialog.open(DeviceFormDialogComponent, { data: { mode: 'create' } as DeviceFormData, width: '680px' });
    ref.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((payload) => {
      if (!payload) return;
      deviceCreateApi(this.http, payload).subscribe({
        next: () => {
          this.snackBar.open('设备登记成功', '关闭', { duration: 2000 });
          this.load();
          this.loadAlerts();
        },
        error: (err) => this.snackBar.open(parseHttpError(err), '关闭', { duration: 3000 }),
      });
    });
  }

  openEdit(d: Device): void {
    const ref = this.dialog.open(DeviceFormDialogComponent, { data: { mode: 'edit', device: d } as DeviceFormData, width: '680px' });
    ref.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((payload) => {
      if (!payload) return;
      deviceUpdateApi(this.http, d.id, payload).subscribe({
        next: () => {
          this.snackBar.open('设备更新成功', '关闭', { duration: 2000 });
          this.load();
          this.loadAlerts();
        },
        error: (err) => this.snackBar.open(parseHttpError(err), '关闭', { duration: 3000 }),
      });
    });
  }

  disable(d: Device): void {
    const ref = this.dialog.open(ConfirmDialogComponent, {
      data: { title: '禁用设备', message: `确认禁用设备「${d.name}」吗？禁用后将退出保修预警清单。`, danger: true } as ConfirmDialogData,
    });
    ref.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((ok) => {
      // store.disable/enable 的 reload 会同时回读台账与保修预警清单（禁用后自动退出预警）。
      if (ok) this.store.disable(d.id);
    });
  }

  enable(d: Device): void {
    const ref = this.dialog.open(ConfirmDialogComponent, {
      data: { title: '启用设备', message: `确认启用设备「${d.name}」吗？` } as ConfirmDialogData,
    });
    ref.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((ok) => {
      if (ok) this.store.enable(d.id);
    });
  }

  formatDate = formatDate;
  moneyLabel = moneyLabel;
  WARRANTY_ALERT = WARRANTY_ALERT;
}
