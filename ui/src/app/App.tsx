import { Navigate, Route, Routes } from 'react-router-dom'
import Shell from './shell/Shell'
import OverviewPage from './pages/OverviewPage'
import BridgesLinesPage from './pages/BridgesLinesPage'
import BridgeDetailPage from './pages/BridgeDetailPage'
import MIDashboardPage from './pages/MIDashboardPage'
import RealtimeUsagePage from './pages/RealtimeUsagePage'
import BridgeUsageDetailPage from './pages/BridgeUsageDetailPage'
import ConfigPage from './pages/ConfigPage'
import ConferenceSettingsPage from './pages/ConferenceSettingsPage'
import UserListPage from './pages/UserListPage'
import UserDetailPage from './pages/UserDetailPage'
import DatabaseSettingsPage from './pages/DatabaseSettingsPage'
import RecordingSettingsPage from './pages/RecordingSettingsPage'
import SettingsLayout from './settings/SettingsLayout'
import SbcSetupPage from './pages/SbcSetupPage'
import ServersPage from './pages/ServersPage'
import ClusterPage from './pages/ClusterPage'
import RequireRole from './auth/RequireRole'

export default function App() {
  return (
    <Shell>
      <Routes>
        <Route path="/" element={<OverviewPage />} />
        <Route path="/bridges" element={<BridgesLinesPage />} />
        <Route path="/bridges/:bridgeId" element={<BridgeDetailPage />} />
        <Route
          path="/mi"
          element={
            <RequireRole allow={['admin', 'operator']}>
              <MIDashboardPage />
            </RequireRole>
          }
        />
        <Route path="/usage" element={<RealtimeUsagePage />} />
        <Route path="/usage/bridge/:bridgeId" element={<BridgeUsageDetailPage />} />
        <Route path="/config" element={<Navigate to="/settings/config" replace />} />
        <Route
          path="/settings"
          element={
            <RequireRole allow={['admin']}>
              <SettingsLayout />
            </RequireRole>
          }
        >
          <Route index element={<Navigate to="/settings/config" replace />} />
          <Route path="config" element={<ConfigPage />} />
          <Route path="users" element={<UserListPage />} />
          <Route path="users/:userId" element={<UserDetailPage />} />
          <Route path="conference" element={<ConferenceSettingsPage />} />
          <Route path="database" element={<DatabaseSettingsPage />} />
          <Route path="recording" element={<RecordingSettingsPage />} />
        </Route>
        <Route
          path="/setup/sbc"
          element={
            <RequireRole allow={['admin']}>
              <SbcSetupPage />
            </RequireRole>
          }
        />
        <Route
          path="/servers"
          element={
            <RequireRole allow={['admin']}>
              <ServersPage />
            </RequireRole>
          }
        />
        <Route
          path="/cluster"
          element={
            <RequireRole allow={['admin', 'operator']}>
              <ClusterPage />
            </RequireRole>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Shell>
  )
}
