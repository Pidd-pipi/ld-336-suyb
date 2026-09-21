import { Component, inject, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatCardModule } from '@angular/material/card';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { PageHeaderComponent } from '../../components/page-header/page-header.component';
import { StatusBadgeComponent } from '../../components/status-badge/status-badge.component';
import { EmptyStateComponent } from '../../components/empty-state/empty-state.component';
import { WarrantyStore } from '../../../stores/warranty.store';
import { WarrantyAlertItem } from '../../../models';
import { WARRANTY_ALERT_TEXT, DEPARTMENTS, DEVICE_CATEGORIES } from '../../../constants/enums';
import { formatDate } from '../../../utils/format';

@Component({
  selector: 'app-warranty-alerts',
  standalone: true,
  imports: [
    MatCardModule,
    CommonModule, ReactiveFormsModule, MatTableModule, MatButtonModule, MatIconModule,
    MatFormFieldModule, MatSelectModule, MatProgressSpinnerModule,
    PageHeaderComponent, StatusBadgeComponent, EmptyStateComponent,
  ],
  template: `
    <app-page-header title="保修到期预警" subtitle="后端按当前日期统一计算剩余天数与分类（已报废、已禁用设备不进入清单）"></app-page-header>
    <form class="filter-bar" [formGroup]="form">
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
        <mat-select formControlName="warranty_type">
          <mat-option value="">全部</mat-option>
          <mat-option *ngFor="let t of alertOptions" [value]="t.value">{{ t.label }}</mat-option>
        </mat-select>
      </mat-form-field>
      <div class="spacer"></div>
      <button mat-flat-button color="primary" (click)="search()"><mat-icon>search</mat-icon> 查询</button>
      <button mat-stroked-button color="primary" (click)="refresh()"><mat-icon>refresh</mat-icon> 刷新回读</button>
    </form>

    <div class="summary">
      <span>共 {{ store.list().length }} 台预警设备</span>
    </div>

    <mat-card>
      <div class="table-wrap">
        <table mat-table [dataSource]="store.list()" class="full-table">
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
          <ng-container matColumnDef="responsible_person">
            <th mat-header-cell *matHeaderCellDef>责任人</th>
            <td mat-cell *matCellDef="let d">{{ d.responsible_person || '-' }}</td>
          </ng-container>
          <ng-container matColumnDef="warranty_expiry">
            <th mat-header-cell *matHeaderCellDef>保修到期日</th>
            <td mat-cell *matCellDef="let d">{{ formatDate(d.warranty_expiry) }}</td>
          </ng-container>
          <ng-container matColumnDef="warranty_days_left">
            <th mat-header-cell *matHeaderCellDef>剩余天数</th>
            <td mat-cell *matCellDef="let d">
              <span [class.days-expired]="d.warranty_days_left < 0" [class.days-due]="d.warranty_days_left >= 0">
                {{ daysText(d) }}
              </span>
            </td>
          </ng-container>
          <ng-container matColumnDef="warranty_type">
            <th mat-header-cell *matHeaderCellDef>预警类型</th>
            <td mat-cell *matCellDef="let d"><app-status-badge [status]="d.warranty_type" [labelMap]="alertText"></app-status-badge></td>
          </ng-container>
          <tr mat-header-row *matHeaderRowDef="columns"></tr>
          <tr mat-row *matRowDef="let row; columns: columns;"></tr>
        </table>
        <app-empty-state *ngIf="!store.loading() && store.list().length === 0" message="当前筛选条件下暂无保修预警设备"></app-empty-state>
        <div class="loading" *ngIf="store.loading()"><mat-spinner diameter="30"></mat-spinner></div>
      </div>
    </mat-card>
  `,
  styles: [`
    .full-table { width: 100%; }
    .loading { display: flex; justify-content: center; padding: 24px; }
    .summary { margin: 0 0 12px; font-size: 13px; color: #616161; }
    .days-expired { color: #c62828; font-weight: 600; }
    .days-due { color: #ef6c00; font-weight: 600; }
  `],
})
export class WarrantyAlertsComponent implements OnInit {
  private fb = inject(FormBuilder);
  store = inject(WarrantyStore);

  alertText = WARRANTY_ALERT_TEXT;
  departments = DEPARTMENTS;
  categories = DEVICE_CATEGORIES;
  alertOptions = Object.entries(WARRANTY_ALERT_TEXT).map(([value, label]) => ({ value, label }));
  columns = ['asset_code', 'name', 'category', 'department', 'responsible_person', 'warranty_expiry', 'warranty_days_left', 'warranty_type'];

  form = this.fb.nonNullable.group({
    department: [''],
    category: [''],
    warranty_type: [''],
  });

  ngOnInit(): void {
    this.load();
  }

  private buildQuery() {
    return {
      department: this.form.value.department || undefined,
      category: this.form.value.category || undefined,
      warranty_type: this.form.value.warranty_type || undefined,
    };
  }

  load(): void {
    this.store.load(this.buildQuery());
  }

  search(): void {
    this.load();
  }

  // 刷新后回读：按当前筛选条件重新请求，展示后端按当前日期重新计算的结果。
  refresh(): void {
    this.load();
  }

  daysText(d: WarrantyAlertItem): string {
    if (d.warranty_days_left < 0) {
      return `已过保 ${-d.warranty_days_left} 天`;
    }
    if (d.warranty_days_left === 0) {
      return '今日到期';
    }
    return `剩余 ${d.warranty_days_left} 天`;
  }

  formatDate = formatDate;
}
