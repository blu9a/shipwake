import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  RouterProvider,
  redirect,
  useParams,
  useRouterState,
} from '@tanstack/react-router';
import { StrictMode, Suspense, use } from 'react';
import { createRoot } from 'react-dom/client';
import './tokens.css';
import './app.css';
import { loadProject, loadSite } from './data';
import { CommitPage } from './pages/CommitPage';
import { MeasurePage } from './pages/MeasurePage';
import { ReleasePage } from './pages/ReleasePage';
import { RunPage } from './pages/RunPage';
import { BranchView, TrunkView } from './pages/Timeline';
import { WatchPage } from './pages/WatchPage';
import { SettingsProvider, useSettings, useT } from './settings';
import { Icon, Loading, Menu, Segmented } from './ui';

function Layout() {
  // A commit shows its file tree beside the diffs, so its column is wider; the top bar follows it.
  const wide = useRouterState({ select: (s) => s.matches.some((m) => m.routeId === '/$project/commit/$sha') });
  return (
    <SettingsProvider>
      <div className={`frame ${wide ? 'wide' : ''}`}>
        <Suspense fallback={<header className="topbar" />}>
          <TopBar />
        </Suspense>
        <main className="page">
          <Suspense fallback={<Loading />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </SettingsProvider>
  );
}

/** The only navigation: project, branch, and the viewer's settings. No sidebar. */
function TopBar() {
  const params = useParams({ strict: false }) as { project?: string; _splat?: string };
  const site = use(loadSite());
  const project = site.projects.find((p) => p.id === params.project) ?? site.projects[0];
  return (
    <header className="topbar">
      <Link to="/" className="brand" aria-label="Shipwake">
        <svg width="22" height="22" viewBox="0 0 32 32" aria-hidden="true">
          <rect width="32" height="32" rx="8" fill="var(--accent)" />
          <path d="M6 21c3 0 4-3 7-3s4 3 7 3 4-3 6-3" fill="none" stroke="var(--on-accent)" strokeWidth="2.4" strokeLinecap="round" />
          <path d="M10 15l6-7 6 7z" fill="var(--on-accent)" />
        </svg>
      </Link>
      {project && <ProjectPicker current={project.id} />}
      {project && (
        <Suspense fallback={null}>
          <BranchPicker project={project.id} />
        </Suspense>
      )}
      <span className="grow" />
      <SettingsMenu />
    </header>
  );
}

function ProjectPicker({ current }: { current: string }) {
  const site = use(loadSite());
  const t = useT();
  const name = site.projects.find((p) => p.id === current)?.name ?? current;
  if (site.projects.length < 2)
    return (
      <span className="crumb" title={name}>
        {name}
      </span>
    );
  return (
    <Menu
      label={t.projects}
      button={
        <>
          <span className="menuLabel" title={name}>
            {name}
          </span>
          <Icon name="down" size={14} />
        </>
      }
    >
      {(close) =>
        site.projects.map((p) => (
          <Link key={p.id} to="/$project" params={{ project: p.id }} className={`menuItem ${p.id === current ? 'on' : ''}`} onClick={close}>
            <span className="menuLabel">{p.name}</span>
          </Link>
        ))
      }
    </Menu>
  );
}

function BranchPicker({ project }: { project: string }) {
  const p = use(loadProject(project));
  const params = useParams({ strict: false }) as { _splat?: string };
  // Only a branch page's address names a branch; a feature page's names a file.
  const onBranch = useRouterState({ select: (st) => st.matches.some((m) => m.routeId === '/$project/branch/$') });
  const t = useT();
  const current = (onBranch && params._splat) || p.trunk;
  return (
    <Menu
      label={t.branch}
      button={
        <>
          <Icon name="branch" />
          <span className="mono menuLabel" title={current}>
            {current}
          </span>
          <Icon name="down" size={14} />
        </>
      }
    >
      {(close) => (
        <>
          <Link to="/$project" params={{ project }} className={`menuItem ${current === p.trunk ? 'on' : ''}`} onClick={close}>
            <Icon name="branch" />
            <span className="mono menuLabel">{p.trunk}</span>
            <small>{t.defaultBranch}</small>
          </Link>
          {p.branches.map((b) => (
            <Link
              key={b.name}
              to="/$project/branch/$"
              params={{ project, _splat: b.name }}
              className={`menuItem ${current === b.name ? 'on' : ''}`}
              onClick={close}
            >
              <Icon name="branch" />
              <span className="mono menuLabel">{b.name}</span>
              <small>{t.commits(b.ahead.length)}</small>
            </Link>
          ))}
        </>
      )}
    </Menu>
  );
}

function SettingsMenu() {
  const { lang, mode, setLang, setMode } = useSettings();
  const t = useT();
  return (
    <Menu label={t.settings} align="end" button={<Icon name="settings" size={18} />}>
      {() => (
        <div className="settings">
          <span>{t.language}</span>
          <Segmented
            value={lang}
            onChange={setLang}
            options={[
              { value: 'en', label: 'English' },
              { value: 'zh', label: '中文' },
            ]}
          />
          <span>{t.appearance}</span>
          <Segmented
            value={mode}
            onChange={setMode}
            options={[
              { value: 'auto', label: t.auto },
              { value: 'light', label: t.light },
              { value: 'dark', label: t.dark },
            ]}
          />
        </div>
      )}
    </Menu>
  );
}

function NotFound() {
  return <p className="muted">404</p>;
}

const rootRoute = createRootRoute({ component: Layout, notFoundComponent: NotFound });
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: async () => {
    const site = await loadSite();
    const first = site.projects[0];
    if (first) throw redirect({ to: '/$project', params: { project: first.id } });
  },
});
const trunkRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project', component: TrunkView });
const branchRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/branch/$', component: BranchView });
const commitRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/commit/$sha', component: CommitPage });
const runRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/run/$run', component: RunPage });
const watchRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/run/$run/watch/$', component: WatchPage });
const measureRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/measure/$run/$metric', component: MeasurePage });
const releaseRoute = createRoute({ getParentRoute: () => rootRoute, path: '$project/release', component: ReleasePage });

const router = createRouter({
  routeTree: rootRoute.addChildren([indexRoute, trunkRoute, branchRoute, commitRoute, runRoute, watchRoute, measureRoute, releaseRoute]),
  history: createHashHistory(),
  scrollRestoration: true,
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

const root = document.getElementById('root');
if (root)
  createRoot(root).render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>,
  );
