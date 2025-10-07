import axios from 'axios';
import type {
  Overview,
  Trade,
  Order,
  Balance,
  Strategy,
  KillSwitchStatus,
  RiskEvent,
  Log,
} from '../types';

const API_BASE_URL = '/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
});

export const getOverview = async (): Promise<Overview> => {
  const { data } = await api.get<Overview>('/overview');
  return data;
};

export const getTrades = async (): Promise<Trade[]> => {
  const { data } = await api.get<Trade[]>('/trades');
  return data;
};

export const getOrders = async (): Promise<Order[]> => {
  const { data } = await api.get<Order[]>('/orders');
  return data;
};

export const getBalances = async (): Promise<Balance[]> => {
  const { data } = await api.get<Balance[]>('/balances');
  return data;
};

export const getStrategy = async (): Promise<Strategy> => {
  const { data } = await api.get<Strategy>('/strategy');
  return data;
};

export const toggleStrategy = async (enabled: boolean): Promise<void> => {
  await api.post('/strategy/toggle', { enabled });
};

export const getKillSwitchStatus = async (): Promise<KillSwitchStatus> => {
  const { data } = await api.get<KillSwitchStatus>('/kill-switch');
  return data;
};

export const enableKillSwitch = async (reason: string): Promise<void> => {
  await api.post('/kill-switch/enable', { reason });
};

export const disableKillSwitch = async (): Promise<void> => {
  await api.post('/kill-switch/disable');
};

export const getRiskEvents = async (): Promise<RiskEvent[]> => {
  const { data } = await api.get<RiskEvent[]>('/risk-events');
  return data;
};

export const getLogs = async (): Promise<Log[]> => {
  const { data } = await api.get<Log[]>('/logs');
  return data;
};

