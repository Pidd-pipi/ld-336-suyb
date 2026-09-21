import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable, map } from 'rxjs';
import { ApiResp, Device, PageResult, WarrantyAlert } from '../models';
import { API_BASE, extractData } from '../utils/request';

export interface DeviceQuery {
  page: number;
  page_size: number;
  department?: string;
  category?: string;
  status?: string;
  keyword?: string;
}

export interface WarrantyAlertQuery {
  department?: string;
  category?: string;
  alert_type?: string;
}

export interface CreateDevicePayload {
  asset_code: string;
  barcode?: string;
  name: string;
  model?: string;
  manufacturer?: string;
  serial_number?: string;
  category?: string;
  department?: string;
  responsible_person?: string;
  location?: string;
  supplier?: string;
  purchase_date?: string;
  purchase_amount?: number;
  warranty_months?: number;
  registration_no?: string;
  certificate_no?: string;
  status?: string;
  calibration_required?: boolean;
}

export type UpdateDevicePayload = Omit<CreateDevicePayload, 'asset_code' | 'barcode' | 'status' | 'purchase_request_id'>;

export function deviceListApi(http: HttpClient, q: DeviceQuery): Observable<PageResult<Device>> {
  let params = new HttpParams().set('page', q.page).set('page_size', q.page_size);
  if (q.department) params = params.set('department', q.department);
  if (q.category) params = params.set('category', q.category);
  if (q.status) params = params.set('status', q.status);
  if (q.keyword) params = params.set('keyword', q.keyword);
  return http.get<ApiResp<PageResult<Device>>>(`${API_BASE}/v1/devices`, { params }).pipe(map(extractData));
}

export function deviceGetApi(http: HttpClient, id: number): Observable<Device> {
  return http.get<ApiResp<Device>>(`${API_BASE}/v1/devices/${id}`).pipe(map(extractData));
}

export function deviceCreateApi(http: HttpClient, payload: CreateDevicePayload): Observable<Device> {
  return http.post<ApiResp<Device>>(`${API_BASE}/v1/devices`, payload).pipe(map(extractData));
}

export function deviceUpdateApi(http: HttpClient, id: number, payload: UpdateDevicePayload): Observable<Device> {
  return http.put<ApiResp<Device>>(`${API_BASE}/v1/devices/${id}`, payload).pipe(map(extractData));
}

export function deviceDisableApi(http: HttpClient, id: number): Observable<Device> {
  return http.post<ApiResp<Device>>(`${API_BASE}/v1/devices/${id}/disable`, {}).pipe(map(extractData));
}

export function deviceEnableApi(http: HttpClient, id: number): Observable<Device> {
  return http.post<ApiResp<Device>>(`${API_BASE}/v1/devices/${id}/enable`, {}).pipe(map(extractData));
}

// 保修到期预警清单：分类（已过保/三十天内到期）与剩余天数全部由后端计算，前端只传过滤条件并展示。
export function deviceWarrantyAlertsApi(http: HttpClient, q: WarrantyAlertQuery): Observable<WarrantyAlert[]> {
  let params = new HttpParams();
  if (q.department) params = params.set('department', q.department);
  if (q.category) params = params.set('category', q.category);
  if (q.alert_type) params = params.set('alert_type', q.alert_type);
  return http.get<ApiResp<WarrantyAlert[]>>(`${API_BASE}/v1/devices/warranty-alerts`, { params }).pipe(map(extractData));
}
