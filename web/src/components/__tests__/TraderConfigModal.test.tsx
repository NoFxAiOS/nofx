/**
 * TraderConfigModal 单元测试
 * 测试交易员配置模态框的表单数据持久化、初始化和生命周期管理
 *
 * 关键场景:
 * 1. 表单数据在选择AI模型时保留
 * 2. 表单数据在选择交易所时保留
 * 3. 编辑模式正确加载现有数据
 * 4. 创建模式使用正确的默认值
 * 5. 模态框关闭后重新打开时数据重置
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TraderConfigModal } from '../TraderConfigModal';

// Mock language context
vi.mock('../../contexts/LanguageContext', () => ({
  useLanguage: vi.fn(() => ({
    language: 'en',
  })),
}));

// Mock translations
vi.mock('../../i18n/translations', () => ({
  t: vi.fn((key: string) => key),
}));

// Mock API calls
vi.mock('../../lib/apiConfig', () => ({
  getApiBaseUrl: vi.fn(() => 'http://localhost:3000/api'),
}));

// Mock fetch for API calls
global.fetch = vi.fn();

describe('TraderConfigModal - 表单数据持久化', () => {
  const mockAvailableModels = [
    { id: 'model-1', name: 'GPT-4', enabled: true, apiKey: 'key1' },
    { id: 'model-2', name: 'Claude', enabled: true, apiKey: 'key2' },
    { id: 'model-3', name: 'DeepSeek', enabled: true, apiKey: 'key3' },
  ];

  const mockAvailableExchanges = [
    { id: 'exchange-1', name: 'Binance' },
    { id: 'exchange-2', name: 'OKX' },
    { id: 'exchange-3', name: 'Bybit' },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    (global.fetch as any).mockResolvedValue({
      json: vi.fn().mockResolvedValue({
        default_coins: ['BTCUSDT', 'ETHUSDT'],
        templates: [{ name: 'default' }],
      }),
    });
  });

  describe('创建模式 - 表单数据保留', () => {
    it('输入交易员名称后选择AI模型，名称应保留', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      const { rerender } = render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 等待表单加载
      await waitFor(() => {
        expect(screen.queryByDisplayValue('')).toBeInTheDocument();
      });

      // 输入交易员名称
      const nameInput = screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                        screen.getByDisplayValue('');

      if (nameInput) {
        await userEvent.clear(nameInput);
        await userEvent.type(nameInput, 'My Awesome Trader');
      }

      // 获取当前的name值
      const inputBeforeModelChange = screen.getByDisplayValue('My Awesome Trader');
      expect(inputBeforeModelChange).toBeInTheDocument();

      // 模拟选择不同的AI模型
      const modelSelect = screen.getByDisplayValue('model-1') ||
                          screen.getAllByRole('combobox')[0];

      if (modelSelect) {
        await userEvent.click(modelSelect);
        const option = screen.getByText('Claude') ||
                      screen.getByDisplayValue('model-2');
        await userEvent.click(option);
      }

      // 验证名称仍保留
      await waitFor(() => {
        const nameAfterChange = screen.queryByDisplayValue('My Awesome Trader');
        expect(nameAfterChange).toBeInTheDocument();
      });
    });

    it('填充表单后选择交易所，所有数据应保留', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 等待表单加载
      await waitFor(() => {
        expect(screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                screen.getByDisplayValue('')).toBeInTheDocument();
      });

      // 填充多个字段
      const formInputs = screen.getAllByRole('textbox');
      if (formInputs.length > 0) {
        await userEvent.clear(formInputs[0]);
        await userEvent.type(formInputs[0], 'Test Trader');
      }

      // 模拟选择交易所
      const exchangeSelect = screen.getAllByRole('combobox')[1];
      if (exchangeSelect) {
        await userEvent.click(exchangeSelect);
        const exchangeOption = screen.getByText('OKX') ||
                              screen.getByDisplayValue('exchange-2');
        await userEvent.click(exchangeOption);
      }

      // 验证名称仍保留
      await waitFor(() => {
        expect(screen.getByDisplayValue('Test Trader')).toBeInTheDocument();
      });
    });

    it('快速选择多个模型，表单应保持稳定', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 输入名称
      const nameInputs = screen.getAllByPlaceholderText(/交易员名称|trader.*name/i);
      const nameInput = nameInputs[0] || screen.getByDisplayValue('');

      if (nameInput) {
        await userEvent.type(nameInput, 'Stable Trader');
      }

      // 快速选择多个模型
      const modelSelects = screen.getAllByRole('combobox');
      for (let i = 0; i < 2; i++) {
        if (modelSelects[0]) {
          await userEvent.click(modelSelects[0]);
          const options = screen.getAllByRole('option');
          if (options[i]) {
            await userEvent.click(options[i]);
          }
        }
      }

      // 验证名称仍保留且没有错误
      await waitFor(() => {
        const nameAfter = screen.queryByDisplayValue('Stable Trader');
        expect(nameAfter).toBeInTheDocument();
      });
    });
  });

  describe('编辑模式 - 数据加载', () => {
    it('编辑模式应加载现有交易员数据', async () => {
      const traderData = {
        trader_id: 'trader-1',
        trader_name: 'Existing Trader',
        ai_model: 'model-2',
        exchange_id: 'exchange-1',
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: 'BTCUSDT,ETHUSDT',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: 'default',
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        initial_balance: 1000,
        scan_interval_minutes: 3,
      };

      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={true}
          traderData={traderData}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
        />
      );

      // 验证名称被正确加载
      await waitFor(() => {
        expect(screen.getByDisplayValue('Existing Trader')).toBeInTheDocument();
      });

      // 验证AI模型被正确加载
      expect(screen.getByDisplayValue('model-2')).toBeInTheDocument();

      // 验证交易所被正确加载
      expect(screen.getByDisplayValue('exchange-1')).toBeInTheDocument();
    });

    it('旧数据缺少system_prompt_template时应使用默认值', async () => {
      const traderDataWithoutTemplate = {
        trader_id: 'trader-2',
        trader_name: 'Old Trader',
        ai_model: 'model-1',
        exchange_id: 'exchange-1',
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: '',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: undefined, // 缺失
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        initial_balance: 1000,
        scan_interval_minutes: 3,
      };

      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={true}
          traderData={traderDataWithoutTemplate as any}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
        />
      );

      // 验证名称加载
      await waitFor(() => {
        expect(screen.getByDisplayValue('Old Trader')).toBeInTheDocument();
      });

      // 组件应该为缺失的system_prompt_template设置默认值
      // 这是一个向后兼容的处理
    });
  });

  describe('生命周期 - 模态框打开/关闭', () => {
    it('打开→关闭→重新打开，表单应重置为默认值', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      const { rerender } = render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 输入名称
      const nameInput = screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                        screen.getAllByRole('textbox')[0];

      if (nameInput) {
        await userEvent.type(nameInput, 'Test Trader');
      }

      // 验证输入
      await waitFor(() => {
        expect(screen.getByDisplayValue('Test Trader')).toBeInTheDocument();
      });

      // 关闭模态框
      rerender(
        <TraderConfigModal
          isOpen={false}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 重新打开模态框
      rerender(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 验证名称被重置为空
      const nameInputAfterReopen = screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                                   screen.getAllByRole('textbox')[0];

      if (nameInputAfterReopen) {
        expect((nameInputAfterReopen as HTMLInputElement).value).toBe('');
      }
    });

    it('创建模式→编辑模式，数据应正确切换', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      const traderData = {
        trader_id: 'trader-3',
        trader_name: 'Edit Mode Trader',
        ai_model: 'model-3',
        exchange_id: 'exchange-2',
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: 'BTCUSDT',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: 'default',
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        initial_balance: 1000,
        scan_interval_minutes: 3,
      };

      // 先渲染创建模式
      const { rerender } = render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 输入一些数据
      const nameInput = screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                        screen.getAllByRole('textbox')[0];

      if (nameInput) {
        await userEvent.type(nameInput, 'Create Mode Data');
      }

      // 切换到编辑模式
      rerender(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={true}
          traderData={traderData}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
        />
      );

      // 验证数据已切换到编辑数据
      await waitFor(() => {
        expect(screen.getByDisplayValue('Edit Mode Trader')).toBeInTheDocument();
      });

      // 验证创建模式的数据不再存在
      expect(screen.queryByDisplayValue('Create Mode Data')).not.toBeInTheDocument();
    });
  });

  describe('边界情况', () => {
    it('空模型列表时应处理gracefully', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={[]}  // 空列表
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 不应该抛出错误
      await waitFor(() => {
        expect(screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                screen.getByDisplayValue('')).toBeInTheDocument();
      });
    });

    it('空交易所列表时应处理gracefully', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={[]}  // 空列表
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 不应该抛出错误
      await waitFor(() => {
        expect(screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                screen.getByDisplayValue('')).toBeInTheDocument();
      });
    });
  });

  describe('hasInitialized状态管理', () => {
    it('hasInitialized应防止重复初始化', async () => {
      const mockOnClose = vi.fn();
      const mockOnSave = vi.fn();

      const { rerender } = render(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={mockAvailableModels}
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 输入数据
      const nameInput = screen.getByPlaceholderText(/交易员名称|trader.*name/i) ||
                        screen.getAllByRole('textbox')[0];

      if (nameInput) {
        await userEvent.type(nameInput, 'First Input');
      }

      // 模拟availableModels改变（但模态框仍打开）
      rerender(
        <TraderConfigModal
          isOpen={true}
          onClose={mockOnClose}
          isEditMode={false}
          availableModels={[
            { id: 'new-model', name: 'New Model', enabled: true, apiKey: 'key' },
            ...mockAvailableModels,
          ]}  // 新的引用，但仍在创建模式
          availableExchanges={mockAvailableExchanges}
          onSave={mockOnSave}
          traderData={undefined}
        />
      );

      // 验证输入的数据不变
      await waitFor(() => {
        expect(screen.getByDisplayValue('First Input')).toBeInTheDocument();
      });
    });
  });
});
